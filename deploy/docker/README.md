# EA QMS Backend - Docker Deployment

This directory contains the standalone Docker Compose configuration for running the Change Control QMS Backend API, PostgreSQL database, and automated database migrations with pre-seeded dev accounts.

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (Windows/macOS) or Docker Engine with Docker Compose v2 (Linux).

## Quick Start

1. **Verify Environment Variables**

   Ensure `.env` exists in this directory (copy from `.env.example` if running from a clean clone):

   ```bash
   cp .env.example .env
   ```

2. **Start the Stack**

   ```bash
   docker compose up -d
   ```

3. **Verify Service Health**

   ```bash
   docker compose ps
   ```

## Services & Ports

| Service | Container Name | Host Port | Description |
|---|---|---|---|
| API | `qms-api` | `http://localhost:1304` | Go REST API Server |
| PostgreSQL | `qms-postgres` | `localhost:5432` | PostgreSQL 16 Alpine |
| Migrator | `qms-migrator` | Internal | Runs Goose migrations (001–008) on boot |

- **Swagger / OpenAPI Documentation:** http://localhost:1304/docs
- **Health Check:** http://localhost:1304/api/healthz

## Pre-Seeded Dev Accounts

All seeded accounts use the default password: `DevPassw0rd!`

| Role | Email | Permissions |
|---|---|---|
| Admin | `admin@eaqms.local` | Full system administration, user management |
| CC Owner | `owner@eaqms.local` | Create, draft, and submit Change Controls |
| Approver | `approver@eaqms.local` | Review and sign off on Change Controls |
| Viewer | `viewer@eaqms.local` | Read-only access to records |

## Stopping & Resetting

**Stop containers:**

```bash
docker compose down
```

**Wipe database and reset seed data (clean slate):**

```bash
docker compose down -v
```
