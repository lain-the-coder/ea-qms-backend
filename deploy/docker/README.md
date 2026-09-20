# EA QMS — Change Control API

This folder runs the Change Control backend on your machine: the API, a
PostgreSQL database, and four test accounts, all set up for you.

**You do not need Go, PostgreSQL, or any development tools.** Only Docker.

---

## What you should have

Unzip the folder. It contains:

```
├── README.md            ← this file
├── .env                 ← settings, already filled in
├── docker-compose.yml   ← the recipe Docker follows
└── sql/schema/          ← eight database files
```

If `.env` is missing, ask me for it — nothing will start without it.

> On macOS and Linux, files starting with a dot are hidden. In Finder press
> **Cmd + Shift + .** to show them.

---

## Step 1 — Install Docker Desktop

Download it for your system:

| | |
|---|---|
| **Windows** | [Docker Desktop for Windows](https://www.docker.com/products/docker-desktop/) |
| **Mac, Apple Silicon** (M1/M2/M3/M4) | [Docker Desktop for Mac — Apple chip](https://www.docker.com/products/docker-desktop/) |
| **Mac, Intel** | [Docker Desktop for Mac — Intel chip](https://www.docker.com/products/docker-desktop/) |
| **Linux** | [Docker Engine](https://docs.docker.com/engine/install/) + [Compose](https://docs.docker.com/compose/install/) |

**Not sure which Mac you have?** Apple menu → About This Mac. If the chip
says "Apple", take the Apple Silicon build.

### During installation

**Windows:** leave **"Use WSL 2 instead of Hyper-V"** ticked. It is the
default and it is the one you want. Windows may ask to restart — let it.

**Mac:** drag Docker to Applications, open it, and allow the privileged
helper when macOS asks.

### Then start it

Open Docker Desktop and wait. The whale icon in your system tray or menu
bar stops animating when it is ready, and the dashboard says
**"Engine running"**.

⚠ **Docker Desktop must be open and running** whenever you use this. It is
not a background service that starts with your computer unless you tell it
to be.

---

## Step 2 — Open a terminal in this folder

**Windows:** open the unzipped folder in File Explorer, click the address
bar, type `powershell` and press Enter.

**Mac:** right-click the folder → Services → **New Terminal at Folder**.
If that is missing, open Terminal, type `cd ` (with a space), then drag the
folder onto the window and press Enter.

**Linux:** right-click inside the folder → **Open in Terminal**.

Check you are in the right place:

```bash
docker compose version
```

That should print a version number. If it says "command not found", Docker
Desktop is not installed or not running.

---

## Step 3 — Start it

```bash
docker compose up -d
```

**The first run takes a few minutes.** It downloads about 100 MB — the
database, the API and a migration tool. Later runs take seconds, because
everything is cached.

You will see lines like:

```
✔ Container qms-postgres  Healthy
✔ Container qms-migrator  Exited
✔ Container qms-api       Started
```

**`qms-migrator Exited` is correct, not an error.** That container's only
job is to create the database tables and then stop.

### Check it worked

Open <http://localhost:1304/api/healthz> in your browser. You should see:

```json
{"status":"ok","database":"connected"}
```

If you see that, everything is running.

---

## Step 4 — Use it

| | |
|---|---|
| **API documentation** | <http://localhost:1304/docs> |
| **Health check** | <http://localhost:1304/api/healthz> |
| **API base URL** | `http://localhost:1304/api` |

The documentation page is interactive — **Try it out** sends real requests
to your own running copy.

### Test accounts

All four use the password **`DevPassw0rd!`**

| Role | Email | What they can do |
|---|---|---|
| Admin | `admin@eaqms.local` | Manage users |
| CC Owner | `owner@eaqms.local` | Create, draft and submit change controls |
| Approver | `approver@eaqms.local` | Approve or reject at both gates |
| Viewer | `viewer@eaqms.local` | Read only |

**To log in through the documentation page:** find `POST /api/login`, click
**Try it out**, put an email and the password in the body, and **Execute**.
Copy the `token` from the response, click the **Authorize** button at the
top of the page, and paste it in.

---

## Everyday commands

Run these from this folder.

**Is it running?**
```bash
docker compose ps
```

**See what it is doing**
```bash
docker compose logs -f
```
Press **Ctrl + C** to stop watching. That does not stop the containers.

**Stop it** — your data is kept
```bash
docker compose down
```

**Start it again**
```bash
docker compose up -d
```

**Start over with a clean, empty database**
```bash
docker compose down -v
docker compose up -d
```
⚠ `-v` deletes the database. Everything you created is gone, and the four
test accounts come back fresh.

---

## If something goes wrong

### "port is already allocated"

Something else on your machine is using port 5432 or 1304 — almost always
PostgreSQL, if you have it installed.

**The simplest fix is to stop your own PostgreSQL** while you use this:

- **Windows:** press Win+R, type `services.msc`, find `postgresql-x64-16`,
  right-click → Stop
- **Mac:** `brew services stop postgresql`
- **Linux:** `sudo service postgresql stop`

**Or change the port instead.** Open `docker-compose.yml`, find:

```yaml
    ports:
      - "5432:5432"
```

and change the **first** number only:

```yaml
    ports:
      - "5433:5432"
```

Then `docker compose down` and `docker compose up -d`. The API is
unaffected — it talks to the database inside Docker, not through that port.

### "Cannot connect to the Docker daemon"

Docker Desktop is not running. Open it and wait for the whale to settle.

### The API container keeps restarting

Check what it is complaining about:

```bash
docker compose logs api
```

Most likely `.env` is missing or a value in it is empty.

### It was working and now it is not

Nothing is ever broken beyond repair here — the whole thing is disposable:

```bash
docker compose down -v
docker compose up -d
```

### Removing it completely

```bash
docker compose down -v
docker rmi 20dumpling/ea-qms-backend:1.2.1 postgres:16-alpine gomicro/goose:latest
```

Then delete the folder. Nothing was installed outside Docker.

---

## What is actually running

Three containers, started in order:

| | What it is | When it stops |
|---|---|---|
| `qms-postgres` | PostgreSQL 16, holding all the data | When you stop it |
| `qms-migrator` | Creates the tables, adds the test accounts | Immediately, on its own |
| `qms-api` | The Go API, listening on port 1304 | When you stop it |

The API waits for the migrator to finish successfully, so by the time you
can reach it the database is fully set up.

Data lives in a Docker **volume** called `docker_pgdata`, not in this
folder. That is why `docker compose down` keeps your data and
`down -v` does not.

---

## A note on `.env`

The `.env` you were given has a development `JWT_SECRET` in it. That is
fine for trying this out on your own machine, and it would not be fine on
a real server. Nothing in it is a real credential.

⚠ **Do not put quotes around any value in that file.** Docker Compose does
not strip them, so `JWT_SECRET="abc"` becomes a secret with quote marks in
it, and logins break in a way that is hard to spot.

---

## More

- **Source code:** [lain-the-coder/ea-qms-backend](https://github.com/lain-the-coder/ea-qms-backend)
- **Docker image:** [20dumpling/ea-qms-backend](https://hub.docker.com/r/20dumpling/ea-qms-backend)
- **Specification and UI prototypes:** [Change-Control-HTML-Design](https://github.com/lain-the-coder/Change-Control-HTML-Design)
