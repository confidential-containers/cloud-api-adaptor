use anyhow::{anyhow, Context, Result};
use attester::{detect_tee_type, Attester, Tee};
use clap::Parser;
use platform::{
    platform_server::{Platform, PlatformServer},
    EvidenceRequest, EvidenceResponse, TeeRequest, TeeResponse,
};
use std::path::Path;
use std::sync::Arc;
use tokio::fs::remove_file;
use tokio::net::UnixListener;
use tokio::sync::Mutex;
use tokio_stream::wrappers::UnixListenerStream;
use tonic::transport::Server;
use tonic::{Request, Response, Status};

pub mod platform {
    tonic::include_proto!("platform");
}

/// Attester daemon
#[derive(Parser, Debug, Clone)]
#[command(author, version, about, long_about = None)]
struct Config {
    /// unix domain socket to listen on
    #[arg(
        short,
        long,
        default_value = "/run/confidential-containers/attester.sock"
    )]
    socket_path: String,
}

struct PlatformService {
    client: Arc<Mutex<TeeClient>>,
}

#[tonic::async_trait]
impl Platform for PlatformService {
    async fn tee(&self, _request: Request<TeeRequest>) -> Result<Response<TeeResponse>, Status> {
        tracing::info!(message = "Tee request");
        let tee = self.client.lock().await.tee();
        let res = TeeResponse { tee };
        Ok(Response::new(res))
    }

    async fn evidence(
        &self,
        request: Request<EvidenceRequest>,
    ) -> Result<Response<EvidenceResponse>, Status> {
        let challenge = request.into_inner().challenge;
        tracing::info!(
            message = "Evidence request",
            challenge = format!("{challenge:x?}")
        );
        let evidence = self
            .client
            .lock()
            .await
            .evidence(challenge)
            .await
            .map_err(|e| Status::internal(e.to_string()))?;
        let res = EvidenceResponse { evidence };
        Ok(Response::new(res))
    }
}

struct TeeClient {
    attester: Box<dyn Attester + Send + Sync>,
    tee: Tee,
}

impl TeeClient {
    fn new() -> Result<Self> {
        let tee = detect_tee_type();
        let attester = tee.to_attester().context("failed to create attester")?;
        Ok(Self { attester, tee })
    }

    fn tee(&self) -> String {
        self.tee.to_string()
    }

    async fn evidence(&self, challenge: Vec<u8>) -> Result<String> {
        let tee_evidence = self
            .attester
            .get_evidence(challenge)
            .await
            .context("failed to collect evidence")?;
        Ok(tee_evidence)
    }
}

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt().init();

    let config = Config::parse();
    let client = TeeClient::new()?;
    let client = Mutex::new(client);
    let client = Arc::new(client);
    let path = &config.socket_path;
    if let Some(parent) = Path::new(path).parent() {
        if !parent.exists() {
            return Err(anyhow!("{} directory does not exist", parent.display()));
        }
    }
    let _ = remove_file(path).await;

    let uds = UnixListener::bind(path)?;
    let uds_stream = UnixListenerStream::new(uds);
    let attester_service = PlatformService { client };

    tracing::info!(message = "Starting server", %path);

    Server::builder()
        .add_service(PlatformServer::new(attester_service))
        .serve_with_incoming(uds_stream)
        .await?;
    Ok(())
}
