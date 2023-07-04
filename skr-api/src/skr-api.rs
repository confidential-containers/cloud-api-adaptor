use anyhow::Context;
use axum::extract::{Path, State};
use axum::{http::StatusCode, routing::get, Router};
use clap::Parser;
use crypto::{hash_chunks, TeeKey};
use kbs_types::Attestation;
use platform::{platform_client::PlatformClient, EvidenceRequest, TeeRequest};
use std::convert::From;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;
use tokio::net::UnixStream;
use tokio::sync::Mutex;
use tonic::transport::{Channel, Endpoint};
use tower::service_fn;
use tower_http::trace::{self, TraceLayer};
use tracing::{error, Level};

const KBS_REQ_TIMEOUT_SEC: u64 = 60;
const KBS_GET_RESOURCE_MAX_ATTEMPT: u64 = 3;

pub mod platform {
    tonic::include_proto!("platform");
}

/// Secure Key Release API
#[derive(Parser, Debug, Clone)]
#[command(author, version, about, long_about = None)]
struct Config {
    /// URI of the KBS to query
    #[arg(short, long, default_value = "http://127.0.0.1:8080")]
    kbs_url: String,

    /// unix domain socket to connect to for getting TEE evidence
    #[arg(
        short,
        long,
        default_value = "/run/confidential-containers/skr-api.sock"
    )]
    socket_path: String,

    /// tcp port to listen on
    #[arg(short, long, default_value = "50080")]
    port: u16,
}

#[derive(serde::Deserialize)]
struct Resource {
    repository_name: String,
    r#type: String,
    tag: String,
}

async fn kbs_auth(
    client: &mut reqwest::Client,
    kbs_url: &str,
    tee: &str,
) -> anyhow::Result<String> {
    let url = format!("{kbs_url}/kbs/v0/auth");
    tracing::info!(message = "Calling kbs", %url);
    let request = kbs_protocol::types::Request::new(tee.into());
    let challenge = client
        .post(url)
        .header("Content-Type", "application/json")
        .json(&request)
        .send()
        .await?
        .json::<kbs_protocol::types::Challenge>()
        .await?
        .nonce;
    Ok(challenge)
}

async fn platform_evidence(
    client: &mut PlatformClient<Channel>,
    nonce: String,
    tee_key: &TeeKey,
) -> anyhow::Result<Attestation> {
    let tee_pubkey = tee_key
        .export_pubkey()
        .context("failed to export TEE pubkey")?;

    let ehd_chunks = vec![
        nonce.clone().into_bytes(),
        tee_pubkey.k_mod.clone().into_bytes(),
        tee_pubkey.k_exp.clone().into_bytes(),
    ];

    let ehd = hash_chunks(ehd_chunks);

    let request = tonic::Request::new(EvidenceRequest { challenge: ehd });
    let tee_evidence = client.evidence(request).await?.into_inner().evidence;
    tracing::info!(message = "Received platform evidence");
    let attestation = Attestation {
        tee_pubkey,
        tee_evidence,
    };

    Ok(attestation)
}

#[derive(serde::Deserialize)]
struct AttestationResponseData {
    token: String,
}

async fn kbs_attest(
    client: &mut reqwest::Client,
    kbs_url: &str,
    attestation: Attestation,
) -> anyhow::Result<String> {
    let url = format!("{kbs_url}/kbs/v0/attest");
    tracing::info!(message = "Calling kbs", %url);
    let response = client
        .post(url)
        .header("Content-Type", "application/json")
        .json(&attestation)
        .send()
        .await?;
    let StatusCode::OK = response.status() else {
        return Err(anyhow::anyhow!("kbs failed to attest the evidence"))
    };
    let token = response.json::<AttestationResponseData>().await?.token;
    Ok(token)
}

async fn kbs_get_token(state: &mut AppState) -> anyhow::Result<String> {
    let kbs_url = &state.config.kbs_url;
    let tee_key = &state.tee_key;
    let tee = &state.tee;
    let http_client = &mut state.http_client;
    let platform_client = &mut state.platform_client;

    let challenge = kbs_auth(http_client, kbs_url, tee).await?;
    let attestation = platform_evidence(platform_client, challenge, tee_key).await?;
    let token = kbs_attest(http_client, kbs_url, attestation).await?;
    Ok(token)
}

async fn kbs_get_resource(state: &mut AppState, resource: Resource) -> anyhow::Result<Vec<u8>> {
    let kbs_url = &state.config.kbs_url;
    let tee_key = state.tee_key.clone();
    let resource_path = format!(
        "{}/{}/{}",
        resource.repository_name, resource.r#type, resource.tag
    );
    let url = format!("{kbs_url}/kbs/v0/resource/{resource_path}");

    let token_mutex = state.token.clone();
    let token = &mut token_mutex.lock().await;

    // If there is no token set, perform an attestation to get one
    if token.is_none() {
        tracing::info!("no token yet, getting one");
        **token = Some(kbs_get_token(state).await?);
    }

    for attempt in 1..=KBS_GET_RESOURCE_MAX_ATTEMPT {
        tracing::info!(message = "Calling kbs", %url, %attempt);
        let response = state.http_client.get(&url).send().await?;
        let status_code = response.status();
        if let StatusCode::OK = status_code {
            let response = response.json::<kbs_protocol::types::Response>().await?;
            let payload_data = response.decrypt_output(tee_key)?;
            return Ok(payload_data);
        }
        // The token might be expired
        if let StatusCode::UNAUTHORIZED = status_code {
            tracing::info!("kbs returned unauthorized, getting new token");
            **token = Some(kbs_get_token(state).await?);
            continue;
        }
        tracing::error!(message = "kbs returned error", %status_code);
    }
    Err(anyhow::anyhow!(
        "kbs failed to retrieve resource {resource_path}"
    ))
}

async fn getresource_handler(
    State(mut state): State<AppState>,
    Path(resource): Path<Resource>,
) -> Result<Vec<u8>, StatusCode> {
    kbs_get_resource(&mut state, resource).await.map_err(|e| {
        error!("failed to retrieve kbs resource: {:?}", e);
        StatusCode::INTERNAL_SERVER_ERROR
    })
}

#[derive(Clone)]
struct AppState {
    config: Config,
    platform_client: PlatformClient<Channel>,
    http_client: reqwest::Client,
    tee: String,
    tee_key: TeeKey,
    token: Arc<Mutex<Option<String>>>,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt().init();
    let http_trace_layer = TraceLayer::new_for_http()
        .make_span_with(trace::DefaultMakeSpan::new().level(Level::INFO))
        .on_response(trace::DefaultOnResponse::new().level(Level::INFO));

    let config = Config::parse();
    let path = config.socket_path.clone();

    // HTTP endpoint is a dummy since we bind server to an UDS
    let channel = Endpoint::try_from("http://[::]:50000")?
        .connect_with_connector(service_fn(move |_| {
            tracing::info!(message = "Connect to server", %path);
            UnixStream::connect(path.clone())
        }))
        .await?;
    let mut platform_client = PlatformClient::new(channel);

    // Get TEE initially
    let request = tonic::Request::new(TeeRequest {});
    let tee = platform_client.tee(request).await?.into_inner().tee;
    tracing::info!(message = "Received platform tee", %tee);

    let http_client = reqwest::Client::builder()
        .cookie_store(true)
        .user_agent(format!(
            "cloud-api-adaptor-skr-api/{}",
            env!("CARGO_PKG_VERSION")
        ))
        .timeout(Duration::from_secs(KBS_REQ_TIMEOUT_SEC))
        .build()?;
    let tee_key = TeeKey::new().ok().context("failed to create TEE key")?;
    let app_state = AppState {
        config: config.clone(),
        platform_client,
        http_client,
        tee,
        tee_key,
        token: Arc::new(Mutex::new(None)),
    };

    let app = Router::new()
        .route(
            "/getresource/:repository_name/:type/:tag",
            get(getresource_handler),
        )
        .route("/health", get(|| async { "OK" }))
        .with_state(app_state)
        .layer(http_trace_layer);

    let addr = SocketAddr::from(([127, 0, 0, 1], config.port));
    axum::Server::bind(&addr)
        .serve(app.into_make_service())
        .await?;

    Ok(())
}
