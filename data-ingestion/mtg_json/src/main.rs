use std::fs::File;
use std::io::copy;
use flate2::read::GzDecoder;
use reqwest::blocking::get;
use tar::Archive;<p></p>
<p>pub fn download_mtg_json() -> Result<(), Box<dyn std::error::Error>> {
    let url = "<a href="https://mtgjson.com/files/AllSets.json.tar.gz">https://mtgjson.com/files/AllSets.json.tar.gz</a>";
    let response = get(url)?;
    let mut tar_gz = GzDecoder::new(response);
    let mut archive = Archive::new(&mut tar_gz);</p>
<pre><code>for file in archive.entries()? {
    let mut file = file?;
    let path = file.path()?;
    let file_name = path.file_name().unwrap().to_str().unwrap();
    let mut output_file = File::create(file_name)?;
    copy(&amp;mut file, &amp;mut output_file)?;
}

Ok(())
</code></pre>
