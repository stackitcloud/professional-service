# Handout — STACKIT-Whisper-functions (laufende Session)

## Status: ÜBERHOLT -- siehe DECISIONS.md und GETTING-STARTED.md

**Dieses Dokument ist historisch.** Der Disk-Space-Blocker unten wurde in
einer späteren Session gelöst (freier Speicher + Docker-RAM erhöht), und das
Projekt ist seit 2026-10-02 **live deployed** -- mit `small` statt `large-v3`
und per Dockerfile statt Buildpacks (siehe `DECISIONS.md` für warum). Für den
aktuellen Stand: `GETTING-STARTED.md`, `DECISIONS.md`, Root-`README.md`. Rest
dieser Datei ist das Session-Log bis zum Blocker, nicht der aktuelle Stand.

Fortlaufendes Log für Session-Übergabe (token-sparend). Konzept/Ausgangslage siehe `Task.md`.
Diese Datei wird nach jedem abgeschlossenen Schritt aktualisiert.

## Entscheidung (durch Nutzer bestätigt)
- Projekt bleibt getrennt von `STACKIT-Whisper-open-ai` (Projektgrenze-Regel dort respektiert).
- **Neuer eigener S3-Bucket** wird angelegt (nicht den alten wiederverwenden). Noch nicht erstellt
  — braucht explizite Bestätigung vor dem Anlegen (Kostenregel).

## Erledigt
1. **`sfn` CLI installiert** — v1.6.5, lokal via offizielles Install-Skript nach `~/.local/bin/sfn`.
   PATH-Hinweis: `export PATH="$HOME/.local/bin:$PATH"` nötig in neuen Shells.
2. **Templates-Repo installiert**: `sfn templates install` → `sfn-templates` (Quelle:
   github.com/stackitcloud/sfn-templates). Enthält `py-template` (Python, HTTP-Trigger, ASGI-Stil).
   → **Kein Dockerfile nötig** — Build läuft über Cloud Native Buildpacks aus `pyproject.toml`.
   Das widerspricht dem ursprünglichen Plan im Task.md (dort war noch von Dockerfile die Rede).
3. **Function bootstrapped**: `sfn functions bootstrap --name whisper-fn --path . --runtime python
   --trigger http` → Ordner `whisper-fn/` mit `handler.py`, `pyproject.toml`,
   `.stackit-functions/` (function.yaml, project.yaml, revision.yaml), `stackit-functions.yaml`.
4. **`handler.py` geschrieben** (ersetzt den Platzhalter aus dem Template):
   - Routen: `GET /health` (Status+Modellname), `POST /transcribe` (Kernlogik)
   - Lifecycle: Whisper-Modell wird in `start()` einmal pro warmer Instanz geladen (nicht pro
     Request) — reduziert Cold-Start-Overhead bei wiederholten Aufrufen
   - Ablauf `/transcribe`: Video aus S3 laden (`S3_BUCKET`/`S3_INPUT_KEY`) → mit `whisper`
     transkribieren → Text-Datei erzeugen → **zurück nach S3 hochladen** (`S3_OUTPUT_KEY`) →
     JSON-Antwort mit Bucket/Keys/Modell/Dauer. Das ist der entscheidende Unterschied zur alten
     VM-Lösung, die das Ergebnis nur lokal auf der VM abgelegt hat.
   - Alle S3-Parameter und `WHISPER_MODEL` (Default: `small`, NICHT `large-v3` — siehe
     Timeout-Risiko in Task.md) über Env-Vars konfigurierbar, kein Hardcoding.
   - **ffmpeg-Problem gelöst**: `openai-whisper` ruft intern das System-Binary `ffmpeg` auf.
     Cloud-Native-Buildpack-Container haben aber keinen `apt-get`-Zugriff → System-ffmpeg wäre
     nicht installierbar gewesen. Lösung: `imageio-ffmpeg` (pip-Paket, bringt eine statische
     ffmpeg-Binary mit) und deren Verzeichnis zur Laufzeit vorne an `$PATH` gehängt. Kein
     System-Paket nötig.
5. **`pyproject.toml` ergänzt** um `boto3`, `openai-whisper`, `imageio-ffmpeg`.
6. **Lokaler Build gestartet**: `sfn functions build --image whisper-fn:local --no-interactive -v`
   im Hintergrund (Docker lokal vorhanden, `docker info` bestätigt). Lädt torch (CPU-Wheel,
   mehrere hundert MB) + whisper — dauert einige Minuten, rein lokal/kostenlos.
   Log: `/tmp/sfn-build.log`.
   **Wichtiger Hinweis für Folge-Session:** Erster Build-Versuch ohne `--image` schlug fehl
   (`Can not interactively ask for image ... or configure an image name in the .stackit-functions/
   manifest file`) — Flag `--image whisper-fn:local` behebt das.

## 🔴 BLOCKER (kritisch, Maschinen-weit) — 04.09., ca. 10:55–11:15 CEST
Lokaler Build (`sfn functions build --image whisper-fn:local`) ist inhaltlich **erfolgreich**
durchgelaufen (siehe `/tmp/sfn-build.log`: alle Deps inkl. torch-2.14.0, openai-whisper-20250625,
imageio-ffmpeg installiert, Image `whisper-fn:local` mit ID `ef3f1efcacca` gespeichert). Direkt
danach schlug das Aufräumen des Builder-Containers fehl mit Docker-`input/output error`
(containerd-Blobstore).

**Root Cause:** Root-Disk des Macs war zwischenzeitlich **komplett voll** (0 Byte frei,
`ENOSPC: no space left on device` sogar bei winzigen Datei-Schreibversuchen). Vermutlich hat der
~2-3 GB Torch/Whisper-Download in der Docker-Desktop-VM-Disk den letzten freien Platz verbraucht
und dabei die containerd-Metadaten-DB korrumpiert. Stand 11:15: wieder ~170-200 Mi frei (Textdateien
gehen wieder), aber Docker-Daemon reagiert nicht mehr zuverlässig — `docker system df` läuft in
den 120s-Timeout, keine Ausgabe.

**Das ist kein Projekt-Problem, sondern betrifft die gesamte Maschine.** Ich habe bewusst
**keine Dateien gelöscht**, um Platz zu schaffen — das ist eine Entscheidung, die der Nutzer
treffen muss (welche Downloads/Altlasten weg können), nicht die KI ungefragt. Ein Neustart von
Docker Desktop würde vermutlich den hängenden Daemon reparieren, wurde aber ebenfalls nicht
eigenständig ausgeführt, da das andere laufende Container/Workloads des Nutzers stören könnte.

**Update 11:20 CEST:** Nutzer hat Docker-Neustart bestätigt. Durchgeführt: `osascript -e 'quit app
"Docker Desktop"'` (sauberer Quit, Prozess war unter "Docker Desktop" registriert, nicht "Docker" —
`killall Docker` lief ins Leere) → `open -a "Docker Desktop"`. Dabei mit-beendet: 4 langlaufende
`docker run hashicorp/terraform-mcp-server`-Container (mehrere Tage alt, von anderen MCP-Tools) —
sollten bei Bedarf automatisch neu starten. Warte aktuell auf Daemon-Bereitschaft (`docker info`).

**Update 12:49 CEST — Neustart hat NICHT geholfen, Ursache endgültig bestätigt:**
Docker Desktop GUI-Prozess lief nach dem Neustart wieder (ab 12:05PM), aber der Daemon selbst
antwortet auch nach >5 Minuten nicht (`docker info`, `docker ps`, `docker system df` — alle drei
liefen unbegrenzt und mussten zwangsbeendet werden). Root-Disk unverändert bei **100 % Auslastung,
170 Mi frei**. Schlussfolgerung: Die Docker-interne Linux-VM (liegt als Diskimage auf derselben
fast vollen Root-Volume) kann mit so wenig Platz nicht sauber starten. **Das ist der Kern des
Blockers, nicht der Docker-Prozess selbst.**

**Session-Fazit:** Weitere Docker-Neustartversuche sind aussichtslos und wurden gestoppt (auch aus
Kostengründen — Session lag bei ~$11.70). Fortschritt kann erst weitergehen, wenn echter
Speicherplatz frei ist (nicht nur Docker-Cache — die Root-Disk selbst ist voll, das ist ein
Maschinen-weites Problem, kein Docker-Problem).

**Update 13:05 CEST — sichtbares Ergebnis OHNE Docker geliefert:**
Da Docker/Disk-Blocker nicht kurzfristig lösbar war, wurde `whisper-fn/test_handler_local.py`
geschrieben: ruft den echten `handler.py`-Code auf (Routing, Fehlerbehandlung, S3-Upload-Flow),
mit `boto3`/`whisper`/`imageio_ffmpeg` als In-Memory-Stubs (kein zusätzlicher Speicherplatz nötig,
kein Docker nötig). Ergebnis — alle 5 Checks bestanden:
```
[PASS] GET /health -> 200 {"status": "ok", "model": "small"}
[PASS] GET /unknown -> 404 {"error": "not_found"}
[PASS] POST /transcribe (no S3_BUCKET) -> 500 {"error": "S3_BUCKET env var not set"}
[PASS] POST /transcribe (fake S3+whisper) -> 200 {"bucket": "fake-bucket", ...}
[PASS] transcript uploaded to fake S3: 'hello from fake whisper'
```
Dabei einen echten kleinen Bug gefunden+behoben: `S3_BUCKET` wird in `handler.py` als
Modul-Konstante beim Import gelesen (für echten Deploy korrekt, da Env-Vars beim Container-Start
fix sind) — der Test musste daher `handler.S3_BUCKET` direkt patchen statt `os.environ` nach dem
Import zu setzen. Handler-Code selbst mmusste nicht geändert werden.
**Was das NICHT beweist:** echte Whisper-Transkription, echte S3-Verbindung, Verhalten im
Buildpack-Container. Das bleibt vom Disk-Space-Blocker abhängig.

**Nächste Schritte für die Folge-Session, SOBALD Speicherplatz wirklich frei ist (deutlich mehr als
170 Mi, mind. einige GB empfohlen wg. weiterer Docker-Builds):**
1. Docker Desktop ggf. erneut neu starten, `docker info` prüfen.
2. `docker images | grep whisper-fn` — evtl. ist das bereits gebaute Image (`whisper-fn:local`,
   ID `ef3f1efcacca`) trotz allem noch da und nutzbar.
3. Falls nicht mehr vorhanden/beschädigt: `sfn functions build --image whisper-fn:local
   --no-interactive -v` erneut laufen lassen (Build-Logik war fehlerfrei, siehe oben/
   `/tmp/sfn-build.log`).
4. `sfn functions run --path .` → `GET http://127.0.0.1:8080/health` als Rauchtest.
5. Erst danach: Nutzer nach STACKIT Project-ID fragen (für `sfn auth login`), neuen S3-Bucket
   anlegen (nur nach expliziter Bestätigung, Kostenregel), Deploy (nur nach explizitem OK).

## Was bereits fertig UND verifiziert im Code ist (unabhängig vom Docker-Blocker)
- `whisper-fn/handler.py` — vollständige Transkriptions-Logik (S3 → Whisper → S3-Upload),
  ffmpeg-Workaround via `imageio-ffmpeg`, Env-var-Konfiguration, Warm-Start-Model-Loading
- `whisper-fn/pyproject.toml` — alle Dependencies ergänzt, wurden im Buildpack-Log erfolgreich
  installiert (Beweis: `Successfully installed ... openai-whisper-20250625 ... torch-2.14.0 ...`)
- `sfn` CLI + Templates lokal eingerichtet, Function-Skeleton bootstrapped

## Noch offen / nächste Schritte
1. **Build-Ergebnis prüfen** (`/tmp/sfn-build.log`, `docker images | grep whisper-fn`).
2. **Lokal laufen lassen & testen**: `sfn functions run --path .` (nutzt das gebaute Image oder
   baut neu) → HTTP-Server auf `127.0.0.1:8080` (Default-Range 8080–8090). Ohne echten S3-Bucket
   schlägt `/transcribe` erwartungsgemäß fehl (kein `S3_BUCKET` gesetzt) — `/health` sollte aber
   200 liefern. Das reicht als lokaler Rauchtest, solange der Bucket noch nicht existiert.
3. **`sfn auth login` — braucht Nutzer-Interaktion**: Browser-Flow, benötigt `--p-id <project-id>`
   oder `--project-name <name>`. Diese Info liegt nur beim Nutzer vor (kein Cross-Projekt-Zugriff
   auf andere STACKIT-Configs erlaubt). → **Frage an Nutzer, sobald es an echtes Deploy geht.**
4. **Neuer S3-Bucket anlegen** — braucht explizite Bestätigung (Kostenregel!). Erst Plan zeigen
   (welcher Service, welche Region, geschätzte Kosten), dann auf "ja, anlegen" warten, genau wie
   beim Terraform-Apply-Muster im alten Projekt.
5. Test-Video `elephants_dream.mp4` in neuen Bucket hochladen (kein Zugriff auf alten Bucket).
6. `sfn functions deploy` — **nur nach explizitem OK**, verursacht STACKIT-Kosten.
7. Nach erfolgreichem Deploy: Transkript in S3 verifizieren, dann Vergleich alte VM-Lösung vs.
   neue Function-Lösung dokumentieren (Task.md Punkt 7).

## Für eine neue/token-arme Session
Alles Nötige steht in `Task.md` (Konzept) + dieser `Handout.md` (Fortschritt). Reicht als Kontext,
ohne die komplette bisherige Conversation neu zu laden. Nächster sinnvoller Einstieg: Punkt 1–2
oben (Build-Log/Ergebnis prüfen, lokal testen), danach Nutzer nach Project-ID fragen (Punkt 3).
