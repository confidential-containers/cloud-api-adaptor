use anyhow::{Context, Result};
use attester::{Attester, Tee};
use axum::extract::{Path, State};
use axum::{http::StatusCode, routing::get, Router};
use clap::Parser;
use kbs_protocol::{KbsProtocolWrapper, KbsProtocolWrapperBuilder, KbsRequest};
use platform::{platform_client::PlatformClient, EvidenceRequest, TeeRequest};
use std::convert::From;
use std::net::SocketAddr;
use std::os::linux::net::SocketAddrExt;
use std::os::unix::net::SocketAddr as UnixSocketAddr;
use std::os::unix::net::UnixStream as StdStream;
use std::sync::Arc;
use tokio::net::UnixStream;
use tokio::sync::Mutex;
use tonic::transport::{Channel, Endpoint};
use tower::service_fn;
use tower_http::trace::{self, TraceLayer};
use tracing::{error, Level};

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

    /// domain socket path to connect to for getting TEE evidence. If the path is prefixed
    /// with @ it denotes an abstract socket.
    #[arg(
        short,
        long,
        default_value = "@/run/confidential-containers/attester.sock"
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

async fn getresource_handler(
    State(state): State<AppState>,
    Path(resource): Path<Resource>,
) -> Result<Vec<u8>, StatusCode> {
    let kbs_url = &state.config.kbs_url;
    let resource_path = format!(
        "{}/{}/{}",
        resource.repository_name, resource.r#type, resource.tag
    );
    let url = format!("{kbs_url}/kbs/v0/resource/{resource_path}");

    let kbs_client = &mut state.kbs_client.lock().await;
    kbs_client.http_get(url).await.map_err(|e| {
        error!("failed to retrieve kbs resource: {:?}", e);
        StatusCode::INTERNAL_SERVER_ERROR
    })
}

#[derive(Clone)]
struct AppState {
    config: Config,
    kbs_client: Arc<Mutex<KbsProtocolWrapper>>,
}

struct IpcAttester {
    platform_client: Box<PlatformClient<Channel>>,
}

#[async_trait::async_trait]
impl Attester for IpcAttester {
    async fn get_evidence(&self, report_data: String) -> Result<String> {
        let request = tonic::Request::new(EvidenceRequest {
            challenge: report_data,
        });
        let tee_evidence = self
            .platform_client
            .clone()
            .evidence(request)
            .await?
            .into_inner()
            .evidence;
        tracing::info!(message = "Received platform evidence");
        Ok(tee_evidence)
    }
}

fn connect_std_stream(path: &str) -> Result<StdStream> {
    let abstract_addr = UnixSocketAddr::from_abstract_name(path.as_bytes())?;
    let std_stream = StdStream::connect_addr(&abstract_addr)?;
    Ok(std_stream)
}

async fn get_tokio_stream(path: String) -> Result<UnixStream> {
    if path.starts_with("@") {
        let path = &path[1..];
        let std_stream = connect_std_stream(path)?;
        let tokio_stream = UnixStream::from_std(std_stream)?;
        return Ok(tokio_stream);
    }
    let stream = UnixStream::connect(path).await?;
    Ok(stream)
}

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt().init();
    let http_trace_layer = TraceLayer::new_for_http()
        .make_span_with(trace::DefaultMakeSpan::new().level(Level::INFO))
        .on_response(trace::DefaultOnResponse::new().level(Level::INFO));

    let config = Config::parse();
    let path = config.socket_path.clone();

    tracing::info!(message = "Connect to server", %path);
    // HTTP endpoint is a dummy since we bind server to an UDS
    let channel = Endpoint::try_from("http://[::]:50000")?
        .connect_with_connector_lazy(service_fn(move |_| get_tokio_stream(path.clone())));

    let mut platform_client = PlatformClient::new(channel);

    // Get TEE initially
    let request = tonic::Request::new(TeeRequest {});
    let tee = platform_client.tee(request).await?.into_inner().tee;
    tracing::info!(message = "Received platform tee", %tee);
    let tee = Tee::try_from(tee.as_str()).context("failed to parse platform tee")?;

    let ipc_attester = Box::new(IpcAttester {
        platform_client: Box::new(platform_client),
    });
    let kbs_protocol_wrapper = KbsProtocolWrapperBuilder::new()
        .with_tee(tee)
        .with_attester(ipc_attester)
        .build()?;

    let app_state = AppState {
        config: config.clone(),
        kbs_client: Arc::new(Mutex::new(kbs_protocol_wrapper)),
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
