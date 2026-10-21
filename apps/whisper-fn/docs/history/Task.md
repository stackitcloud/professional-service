# Task: STACKIT-Whisper-open-ai → STACKIT Functions

## Ziel
Die bestehende, funktionierende Lösung in `~/Projects/STACKIT-Whisper-open-ai/` (Terraform-VM,
läuft dauerhaft, führt beim Boot per cloud-init eine feste Whisper-Transkription aus) als
**STACKIT Function** (Container, HTTP-getriggert, scale-to-zero) neu bauen — kein Dauer-VM mehr.

Quelle für STACKIT Functions: https://docs.functions.onstackit.cloud/getting-started/install.html

## Status: ÜBERHOLT -- siehe DECISIONS.md und GETTING-STARTED.md

**Dieses Dokument ist historisch.** Alle unten offenen Punkte (S3-Credentials,
Plan-Größe, Timeout-Risiko) sind seit 2026-10-02 entschieden und das Projekt
ist **live deployed**. Aktueller Stand: `GETTING-STARTED.md` (wie es läuft),
`DECISIONS.md` (was entschieden wurde und warum), Root-`README.md` (Status +
Live-URL). Rest dieser Datei ist die ursprüngliche Planungsgrundlage, nicht
der aktuelle Stand.

---

## Was die alte Lösung macht (Referenz, NICHT verändern/anfassen ohne Erlaubnis)
- `~/Projects/STACKIT-Whisper-open-ai/stackit/` — Terraform: Ubuntu-VM + S3-Bucket
- `cloud-init.sh` beim ersten Boot:
  1. apt: `ffmpeg`, `python3-pip`
  2. pip: `torch` (CPU), `openai-whisper`, `boto3`
  3. lädt `elephants_dream.mp4` aus S3-Bucket
  4. transkribiert mit `whisper --model large-v3 --language en`
  5. Ergebnis liegt nur lokal auf der VM (`/root/transcription/elephants_dream.txt`)
- **Wichtig:** Dieses alte Projekt hat eine strikte Projektgrenze-Regel (eigene CLAUDE.md):
  keine Credentials/Ressourcen von dort ungefragt übernehmen.

## Was STACKIT Functions kann (aus den Docs recherchiert)
- Container-basiert, `linux/amd64`, HTTP 1.1-Server auf Port aus `$PORT`-Env, non-root `USER`
- Scale-to-zero automatisch, kein manuelles "Herunterfahren" nötig
- Pläne: RAM konfigurierbar (Start 128 MB), **1 shared vCPU** pro Instanz
- Deploy-Flow: `sfn function bootstrap` → `sfn function build --push` → `sfn function deploy`
- Env-Vars/Secrets: fixed / env / dotenv / file (Secret-Manager-Anbindung laut Docs "coming soon")
- **Kein dokumentiertes Request-Timeout-Limit gefunden** (Risiko, s.u.)
- **Kein natives S3-Trigger-Feature** — Auslösung nur per HTTP-Call
- Auth: `sfn auth login --project-id <id>` (Browser) oder mit Service-Account-Key für CI/CD

## Vereinbartes Design (mit Nutzer abgestimmt)
Neuer Ordner: `~/Projects/STACKIT-Whisper-functions/`
- `Dockerfile` — Python-Image, installiert `ffmpeg`, `openai-whisper`, `boto3`, startet HTTP-Server
- `app.py` — minimaler HTTP-Server (Flask/FastAPI). Bei Aufruf:
  1. Video aus S3-Bucket laden (wie bisher: `elephants_dream.mp4`)
  2. Mit `whisper` transkribieren
  3. **Neu ggü. VM-Version:** Transkript zurück nach S3 hochladen (nicht nur lokal ablegen)
  4. HTTP 200 zurückgeben — Function skaliert danach selbst auf 0
- Deploy per `sfn` CLI — **`sfn function deploy` nur nach explizitem OK des Nutzers** (Kostenregel)

## Offene Punkte / Risiken (noch zu klären)
1. **Timeout-Risiko:** Kein dokumentiertes Request-Timeout. `large-v3` auf CPU dauert mehrere
   Minuten → könnte an einem Gateway-Timeout scheitern. Vorschlag: erstmal mit `base` oder
   `small` starten, später ggf. hochskalieren. Noch nicht final entschieden.
2. **Plan-Größe:** `large-v3` braucht ~10 GB RAM, Standard-Plan startet bei 128 MB. Muss im
   STACKIT-Portal/den Docs geprüft werden, welcher Plan groß genug ist (und was er kostet).
3. **S3-Credentials — NOCH NICHT ENTSCHIEDEN:**
   - Option A: bestehenden Bucket/Keys aus `STACKIT-Whisper-open-ai` wiederverwenden
     (braucht explizite Erlaubnis, da Projektgrenze-Regel im alten Projekt)
   - Option B: neuer eigener S3-Bucket + neue Zugangsdaten nur für dieses Projekt
   - **→ Diese Frage zuerst klären, wenn wir weitermachen.**
4. Kein natives S3-Event-Trigger vorhanden — Function wird einfach per HTTP-Request angestoßen
   (reicht für den Demo-Case, ist aber kein "lade Video hoch → automatisch transkribiert").

## Nächste Schritte (nach Klärung von Punkt 3)
1. S3-Credentials-Entscheidung treffen
2. `sfn` CLI installieren (`~/Projects/STACKIT-Whisper-functions/` — lokal, kostenlos)
3. `sfn auth login --project-id <id>` (Browser-Flow, kostenlos, nur Login)
4. Dockerfile + app.py schreiben und **lokal** testen (`sfn function build` + `sfn function run`
   — läuft lokal, keine Cloud-Kosten)
5. Erst nach explizitem OK: `sfn function build --push` + `sfn function deploy` (verursacht
   STACKIT-Kosten laut Kostenregel — nicht eigenständig ausführen)
6. Transkriptions-Ergebnis in S3 verifizieren
7. Vergleich alte VM-Lösung vs. neue Function-Lösung dokumentieren, dann altes Projekt ggf.
   `terraform destroy`-en (nur nach Bestätigung)

## Kostenhinweis (aus globaler Regel)
Kein `sfn function deploy`, kein Anlegen von STACKIT-Ressourcen (Bucket, Function-Instanz)
ohne explizite Bestätigung. CLI-Install, lokales Bauen/Testen des Containers und `sfn auth login`
sind unkritisch.
