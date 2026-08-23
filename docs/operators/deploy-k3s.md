# BlockMesh K3s Deployment Guide

> **Version:** 1.0
> **Last Updated:** 2026-08-23
> **Environment:** Remote K3s Cluster via GitHub Actions

For single-server Docker Compose deployments, see [deploy.md](deploy.md).

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Required Secrets](#required-secrets)
4. [Architecture](#architecture)
5. [Deployment Order](#deployment-order)
6. [Cleanup Order](#cleanup-order)
7. [Workflow Reference](#workflow-reference)
8. [Troubleshooting](#troubleshooting)
9. [Manual Commands](#manual-commands)

---

## Overview

This repository contains GitHub Actions workflows for deploying the BlockMesh application stack to a remote K3s cluster. All workflows are triggered manually via `workflow_dispatch` and use SSH to connect to a remote server where K3s is running.

The stack consists of:
- **PostgreSQL** — Primary database
- **Redis** — Caching and session store
- **Gateway** — Backend API service
- **Web** — Frontend web application

---

## Prerequisites

Before deploying, ensure the following:

- [ ] A remote server with K3s installed and running
- [ ] Helm 3 installed on the remote server
- [ ] Docker installed on the remote server
- [ ] An Ingress controller (e.g., Traefik) installed on K3s
- [ ] DNS records pointing to your K3s node:
  - `blockmesh.nimbusurf.com`
  - `api.blockmesh.nimbusurf.com`
- [ ] GitHub repository secrets configured (see below)
- [ ] The `.helm/` directory in your repository contains valid Helm charts for:
  - `gateway`
  - `web`
  - `postgresql`
  - `redis`

---

## Required Secrets

Configure the following secrets in **Settings → Secrets and variables → Actions**:

| Secret | Required | Description |
|--------|----------|-------------|
| `REMOTE_SERVER` | ✅ | IP address or hostname of the remote K3s server |
| `REMOTE_USER` | ✅ | SSH username for the remote server |
| `SSH_PRIVATE_KEY` | ✅ | Private key for SSH authentication (PEM format) |
| `REMOTE_DEPLOY_DIR` | ✅ | Base directory on the remote server for deployment workspaces (e.g., `/opt/deployments`) |
| `DATABASE_USER` | ✅ | PostgreSQL database username |
| `DATABASE_PASSWORD` | ✅ | PostgreSQL database password |
| `REDIS_PASSWORD` | ✅ | Redis authentication password |
| `ADMIN_SECRET` | ✅ | Admin secret key for the application |

> **Security Note:** Never commit secrets to the repository. Always use GitHub Secrets.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         K3s Cluster                         │
│                     Namespace: blockmesh                    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │   Ingress    │    │   Ingress    │    │   Ingress    │  │
│  │  blockmesh.  │    │ api.blockmesh│    │   (internal) │  │
│  │ nimbusurf.com│    │.nimbusurf.com│    │              │  │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘  │
│         │                   │                    │          │
│  ┌──────▼───────┐    ┌──────▼───────┐          │          │
│  │   Web App    │    │   Gateway    │          │          │
│  │ blockmesh-web│    │blockmesh-gw  │          │          │
│  │   :8080      │    │   :8080      │          │          │
│  └──────────────┘    └──────┬───────┘          │          │
│                             │                  │          │
│              ┌──────────────┼──────────┐      │          │
│              │              │          │      │          │
│       ┌──────▼──────┐ ┌────▼─────┐ ┌──▼──────┴──┐      │
│       │ PostgreSQL  │ │  Redis   │ │ blockmesh  │      │
│       │blockmesh-pg │ │blockmesh │ │  -secrets  │      │
│       │   :5432     │ │  -redis  │ │  (Secret)  │      │
│       └─────────────┘ └──────────┘ └────────────┘      │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Component Details

| Component | Release Name | Deployment Name | Service Port | Domain |
|-----------|-------------|-----------------|--------------|--------|
| PostgreSQL | `postgresql-release` | `blockmesh-postgresql` | `5432` | — |
| Redis | `redis-release` | `blockmesh-redis` | `6379` | — |
| Gateway | `gateway-release` | `blockmesh-gateway` | `8080` | `api.blockmesh.nimbusurf.com` |
| Web | `web-release` | `blockmesh-web` | `80` | `blockmesh.nimbusurf.com` |

---

## Deployment Order

> **⚠️ Important:** Always deploy in the exact order below. Each service depends on the ones before it.

### Step 1: Deploy Secrets
**Workflow:** `k3s-secrets-deploy.yml`

This must run first. It creates the `blockmesh` namespace and the `blockmesh-secrets` Secret containing all sensitive credentials.

**What it does:**
- Creates the `blockmesh` namespace if it doesn't exist
- Creates/updates the `blockmesh-secrets` Secret with:
  - `DATABASE_USER`
  - `DATABASE_PASSWORD`
  - `POSTGRES_DB=blockmesh`
  - `REDIS_PASSWORD`
  - `ADMIN_SECRET`

**To run:**
Go to **Actions → K3s Secrets Deploy → Run workflow**

---

### Step 2: Deploy PostgreSQL
**Workflow:** `postgresql-deploy.yml`

**Depends on:** Step 1 (K3s Secrets Deploy)

Deploys the PostgreSQL database using a custom Helm chart.

**What it does:**
- Copies `.helm/postgresql/` to the remote server
- Installs/upgrades the `postgresql-release` Helm chart
- Waits for the deployment to be available

**Data Persistence:**
- The PVC `blockmesh-postgresql-pvc` is created on first install
- The PVC is **NOT** deleted during cleanup to preserve data

**To run:**
Go to **Actions → PostgreSQL Deploy → Run workflow**

---

### Step 3: Deploy Redis
**Workflow:** `redis-deploy.yml`

**Depends on:** Step 1 (K3s Secrets Deploy)

Deploys the Redis cache using a custom Helm chart.

**What it does:**
- Copies `.helm/redis/` to the remote server
- Installs/upgrades the `redis-release` Helm chart
- Waits for the deployment to be available

**Data Persistence:**
- The PVC `blockmesh-redis-pvc` is created on first install
- The PVC is **NOT** deleted during cleanup to preserve data

**To run:**
Go to **Actions → Redis Deploy → Run workflow**

---

### Step 4: Deploy Gateway
**Workflow:** `gateway-deploy.yml`

**Depends on:** Step 2 (PostgreSQL), Step 3 (Redis), Step 1 (Secrets)

Builds and deploys the BlockMesh Gateway (backend API service).

**What it does:**
- Copies `.helm/gateway/` and `backend/` to the remote server
- Builds Docker image: `blockmesh-gateway:<build-id>`
- Imports the image into K3s containerd runtime
- Installs/upgrades the `gateway-release` Helm chart
- Sets ingress host to `api.blockmesh.nimbusurf.com`
- Waits for deployment to be available
- **Auto-rollback:** If the deployment fails, it prints logs and rolls back the Helm release

**Image Strategy:**
- Uses `image.pullPolicy=Never` (relies on locally imported images)
- Tags images with `github.run_id-github.run_number` for traceability

**To run:**
Go to **Actions → Gateway Deploy → Run workflow**

---

### Step 5: Deploy Web
**Workflow:** `web-deploy.yml`

**Depends on:** Step 4 (Gateway)

Builds and deploys the BlockMesh Web frontend.

**What it does:**
- Copies `.helm/web/` and `web/` to the remote server
- Builds Docker image: `blockmesh-web:<build-id>`
- Imports the image into K3s containerd runtime
- Installs/upgrades the `web-release` Helm chart
- Sets ingress host to `blockmesh.nimbusurf.com`
- Waits for deployment to be available
- **Auto-rollback:** If the deployment fails, it prints logs and rolls back the Helm release

**To run:**
Go to **Actions → Web Deploy → Run workflow**

---

## Cleanup Order

To tear down the stack, run cleanup workflows in **reverse deployment order**:

| Order | Workflow | What It Removes | What It Preserves |
|-------|----------|----------------|-------------------|
| 1 | `web-cleanup.yml` | Helm release `web-release`, local Docker image | — |
| 2 | `gateway-cleanup.yml` | Helm release `gateway-release`, local Docker image | — |
| 3 | `redis-cleanup.yml` | Helm release `redis-release` | PVC `blockmesh-redis-pvc` |
| 4 | `postgresql-cleanup.yml` | Helm release `postgresql-release` | PVC `blockmesh-postgresql-pvc` |
| 5 | `k3s-secrets-cleanup.yml` | Secret `blockmesh-secrets` | — |

> **⚠️ Data Warning:** PostgreSQL and Redis cleanup workflows intentionally preserve PVCs to prevent accidental data loss. If you need a complete wipe including data, see [Manual Commands](#manual-commands).

---

## Workflow Reference

### Deploy Workflows

| Workflow | File | Trigger | Purpose |
|----------|------|---------|---------|
| K3s Secrets Deploy | `k3s-secrets-deploy.yml` | `workflow_dispatch` | Deploy secrets to K3s |
| PostgreSQL Deploy | `postgresql-deploy.yml` | `workflow_dispatch` | Deploy PostgreSQL via Helm |
| Redis Deploy | `redis-deploy.yml` | `workflow_dispatch` | Deploy Redis via Helm |
| Gateway Deploy | `gateway-deploy.yml` | `workflow_dispatch` | Build & deploy backend API |
| Web Deploy | `web-deploy.yml` | `workflow_dispatch` | Build & deploy frontend |

### Cleanup Workflows

| Workflow | File | Trigger | Purpose |
|----------|------|---------|---------|
| K3s Secrets Cleanup | `k3s-secrets-cleanup.yml` | `workflow_dispatch` | Remove secrets from K3s |
| PostgreSQL Cleanup | `postgresql-cleanup.yml` | `workflow_dispatch` | Remove PostgreSQL release |
| Redis Cleanup | `redis-cleanup.yml` | `workflow_dispatch` | Remove Redis release |
| Gateway Cleanup | `gateway-cleanup.yml` | `workflow_dispatch` | Remove Gateway release & image |
| Web Cleanup | `web-cleanup.yml` | `workflow_dispatch` | Remove Web release & image |

---

## Troubleshooting

### Deployment hangs at `kubectl wait`

**Symptom:** Workflow fails with timeout waiting for deployment.

**Check:**
1. View the workflow logs — Gateway and Web deployments automatically print the last 50 logs on failure
2. Check pod status manually:
   ```bash
   kubectl get pods -n blockmesh
   kubectl describe pod <pod-name> -n blockmesh
   ```
3. Common causes:
   - Image pull errors (check `k3s ctr images ls | grep blockmesh`)
   - Missing secrets (verify `blockmesh-secrets` exists)
   - Resource constraints on the node

### Image not found in K3s

**Symptom:** Pod status shows `ImagePullBackOff` or `ErrImageNeverPull`.

**Fix:**
```bash
# On the remote server, verify the image exists:
k3s ctr images ls | grep blockmesh

# If missing, manually import:
docker save blockmesh-gateway:<tag> | k3s ctr images import -
```

### Helm rollback triggered

**Symptom:** Gateway or Web deployment fails and automatically rolls back.

**Steps:**
1. Check the workflow logs for the error
2. Fix the underlying issue (e.g., application code, missing env vars)
3. Re-run the deployment workflow

### Secret missing after deployment

**Symptom:** PostgreSQL or Redis pods fail to start.

**Fix:**
```bash
kubectl get secret blockmesh-secrets -n blockmesh
```
If missing, re-run **K3s Secrets Deploy**.

### Ingress not routing traffic

**Symptom:** Domain returns 404 or connection refused.

**Check:**
1. DNS resolution: `nslookup blockmesh.nimbusurf.com`
2. Ingress status: `kubectl get ingress -n blockmesh`
3. Ingress controller: `kubectl get pods -n kube-system | grep traefik`
4. Service endpoints: `kubectl get svc -n blockmesh`

### SSH connection failures

**Symptom:** Workflow fails during SSH setup or file copy.

**Check:**
- `REMOTE_SERVER` is reachable from GitHub Actions runners
- `SSH_PRIVATE_KEY` has correct permissions (the workflow sets `chmod 600`)
- The public key is in `~/.ssh/authorized_keys` on the remote server
- `REMOTE_USER` has permission to access the `REMOTE_DEPLOY_DIR`

---

## Manual Commands

### Check Stack Status

```bash
# View all resources in the namespace
kubectl get all -n blockmesh

# View pods
kubectl get pods -n blockmesh

# View services
kubectl get svc -n blockmesh

# View ingress
kubectl get ingress -n blockmesh

# View secrets
kubectl get secrets -n blockmesh

# View PVCs
kubectl get pvc -n blockmesh
```

### View Logs

```bash
# Gateway logs
kubectl logs -n blockmesh -l app=blockmesh-gateway --tail=100

# Web logs
kubectl logs -n blockmesh -l app=blockmesh-web --tail=100

# PostgreSQL logs
kubectl logs -n blockmesh -l app=blockmesh-postgresql --tail=100

# Redis logs
kubectl logs -n blockmesh -l app=blockmesh-redis --tail=100
```

### Rollback a Release Manually

```bash
# List releases
helm list -n blockmesh

# Rollback gateway to previous revision
helm rollback gateway-release -n blockmesh

# Rollback web to previous revision
helm rollback web-release -n blockmesh
```

### Complete Data Wipe (⚠️ Destructive)

> **Warning:** This will delete all data including PVCs.

```bash
# 1. Delete all releases
helm uninstall web-release -n blockmesh
helm uninstall gateway-release -n blockmesh
helm uninstall redis-release -n blockmesh
helm uninstall postgresql-release -n blockmesh

# 2. Delete secrets
kubectl delete secret blockmesh-secrets -n blockmesh

# 3. Delete PVCs (DATA LOSS)
kubectl delete pvc blockmesh-postgresql-pvc -n blockmesh
kubectl delete pvc blockmesh-redis-pvc -n blockmesh

# 4. Delete namespace (optional)
kubectl delete namespace blockmesh
```

### Clean Up Local Docker Images

```bash
# On the remote server
docker rmi blockmesh-gateway:latest blockmesh-web:latest
docker system prune -f
k3s ctr images rm docker.io/library/blockmesh-gateway:latest
k3s ctr images rm docker.io/library/blockmesh-web:latest
```

---

## Quick Start Checklist

Use this checklist when deploying a fresh environment:

- [ ] Configure all [Required Secrets](#required-secrets) in GitHub
- [ ] Verify remote server has K3s, Helm, and Docker installed
- [ ] Verify DNS records point to the K3s node
- [ ] Run **K3s Secrets Deploy**
- [ ] Run **PostgreSQL Deploy**
- [ ] Run **Redis Deploy**
- [ ] Run **Gateway Deploy**
- [ ] Run **Web Deploy**
- [ ] Verify all pods are running: `kubectl get pods -n blockmesh`
- [ ] Verify ingress: `kubectl get ingress -n blockmesh`
- [ ] Test domains in browser:
  - `https://blockmesh.nimbusurf.com`
  - `https://api.blockmesh.nimbusurf.com`

---

## Support

For issues or questions:
1. Check the [Troubleshooting](#troubleshooting) section
2. Review workflow logs in GitHub Actions
3. Check K3s and pod logs on the remote server

---

*Generated for the BlockMesh project. Deploy with care.*
