# Smarter OLJ Release Notes

## MVP Download Build

Smarter OLJ now runs as a local-first downloadable app:

- Creates a local SQLite database automatically.
- Serves the web UI from the app binary.
- Opens the browser automatically on startup.
- Lets users paste or upload a CV profile.
- Supports Ollama, OpenAI, Anthropic, and OpenAI-compatible model providers.
- Keeps UI-entered API keys in memory only for the current app session.
- Scrapes OnlineJobsPH from the local UI and queues useful jobs for analysis.
- Exports matched jobs to CSV or XLSX.

## First Run

Download the archive for your operating system, extract it, then run the `smarter-olj` binary.

On Windows, double-click `smarter-olj.exe`.

The app opens `http://localhost:8080` automatically. If the browser does not open, open that URL manually.

## Notes

- Smarter OLJ is independent software and is not affiliated with or endorsed by OnlineJobsPH.
- Cloud AI providers may receive CV text and job descriptions when selected.
- Ollama remains the local privacy-friendly default.
- API keys entered through the UI are not stored in SQLite.
