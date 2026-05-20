use fantoccini::{ClientBuilder, Locator};

#[tokio::main]
async fn main() -> Result<(), fantoccini::error::CmdError> {
    let endpoint = std::env::var("CAMOUFOX_BIDI_ENDPOINT")
        .expect("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session");
    let client = ClientBuilder::native().connect(endpoint).await?;
    client.goto("https://example.com").await?;
    let heading = client.find(Locator::Css("h1")).await?.text().await?;
    println!("{heading}");
    client.close().await
}
