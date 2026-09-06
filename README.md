# Automated CI/CD Pipeline for Go on Kubernetes

This project is a small Go application deployed to Kubernetes through an automated CI/CD pipeline.

The main goal of the project was not to build a complicated application. The goal was to understand and implement the full delivery process around an application:

---

## What the application does

The application is a small Go HTTP API.

It exposes endpoints such as:

```text
GET /health
GET /ready
GET /api/hello
```

The `/health` and `/ready` endpoints are especially useful because Kubernetes uses them as probes.

---

# Architecture

The final deployment flow looks like this:

```text
Developer
    |
    | git push
    v
GitHub Repository
    |
    v
GitHub Actions
    |
    |-- gofmt
    |-- go vet
    |-- go test
    |-- build Go binary
    |-- build Docker image
    |-- security scan
    |
    v
GitHub Container Registry
    |
    | image tagged with Git commit SHA
    v
ghcr.io/bu3lii/cicd-go-k8s:<commit-sha>

GitHub Actions
    |
    | updates Helm image tag
    v
Git Repository
    |
    v
Argo CD
    |
    | detects Git change
    v
Helm
    |
    v
Kubernetes
    |
    v
Go API Pods
```

---

# Step 1: Building the Go API

The first part of the project was just a normal Go application.

The source code was separated into an application entry point and HTTP handlers.

Example structure:

```text
cmd/
└── api/
    └── main.go

internal/
└── handlers/
    ├── handlers.go
    └── handlers_test.go
```

The application listens on port `8080`.

We also added tests using Go's built-in testing tools.

The important local commands were:

```bash
go fmt ./...
go vet ./...
go test ./...
```

These later became part of the CI pipeline.

---

# Step 2: Docker

Next, the Go application was packaged into a Docker image.

We used a multi-stage Docker build.

The first stage contains the Go compiler and builds the application:

```dockerfile
FROM golang:1.26-alpine AS builder
```

The second stage contains only what is required to run the compiled application.

This means the Go compiler and build tools do not need to exist in the final runtime image.

The application was built locally with:

```bash
docker build -t cicd-go-k8s:local .
```

and run with:

```bash
docker run --rm -p 8080:8080 cicd-go-k8s:local
```

At this stage, the application was running inside Docker but not Kubernetes yet.

---

# Step 3: Kubernetes

Next, a local Kubernetes cluster was started using Colima.

The application was deployed using two Kubernetes resources:

```text
Deployment
Service
```

The Deployment describes how the application should run.

For example:

```yaml
replicas: 2
```

means Kubernetes should keep two copies of the application running.

We tested Kubernetes self-healing by manually deleting one pod.

Kubernetes noticed that only one pod remained even though the Deployment requested two replicas.

It automatically created another pod.

This demonstrated an important Kubernetes concept:

```text
desired state != actual state

Kubernetes controller notices

actual state is changed until it matches desired state
```

This process is called reconciliation.

---

# Step 4: Kubernetes Service

The application pods were placed behind a Kubernetes Service.

The Service exposed port `80` and forwarded traffic to port `8080` inside the containers.

```text
Service port 80
      |
      v
Container port 8080
```

Because the Service type was `ClusterIP`, it was only accessible inside the Kubernetes cluster.

For local testing we used:

```bash
kubectl port-forward service/go-api 8080:80
```

Then the application could be accessed at:

```text
http://localhost:8080
```

---

# Step 5: Kubernetes Health Checks

The Kubernetes Deployment uses the Go endpoints we created earlier.

The liveness probe calls:

```text
/health
```

The readiness probe calls:

```text
/ready
```

If the liveness check fails repeatedly, Kubernetes can restart the container.

If the readiness check fails, Kubernetes temporarily stops sending traffic to that pod.

This is useful during deployments or application startup.

---

# Step 6: Helm

Originally, the Kubernetes configuration was written as normal YAML.

For example:

```text
kubernetes/
├── deployment.yaml
└── service.yaml
```

The problem is that values such as the image tag, replica count, and ports were hardcoded.

Helm was introduced to turn the Kubernetes configuration into reusable templates.

The Helm chart lives under:

```text
helm/go-api/
```

The chart includes files such as:

```text
Chart.yaml
values.yaml
templates/
├── deployment.yaml
└── service.yaml
```

`values.yaml` contains configurable values.

Example:

```yaml
replicaCount: 2

image:
  repository: ghcr.io/bu3lii/cicd-go-k8s
  tag: latest
```

The Deployment template references those values:

```yaml
replicas: {{ .Values.replicaCount }}
```

and:

```yaml
image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
```

This means the Kubernetes template does not need to change every time a new Docker image is deployed.

Helm simply renders a new Kubernetes manifest using different values.

We checked the chart using:

```bash
helm lint helm/go-api
```

and rendered the final Kubernetes YAML with:

```bash
helm template go-api helm/go-api
```

---

# Step 7: Rolling Deployments

We built a second application version and updated the Docker image.

When the Helm release was upgraded, Kubernetes performed a rolling deployment.

Instead of stopping both existing pods immediately, Kubernetes did something similar to:

```text
old pod running
old pod running

        ↓

new pod starting
old pod running
old pod running

        ↓

new pod ready
new pod starting
old pod running

        ↓

new pod ready
new pod ready

        ↓

old pods removed
```

This reduces downtime during application updates.

We could watch this happen using:

```bash
kubectl get pods -w
```

---

# Step 8: GitHub Actions CI

The next step was automating the build process.

A GitHub Actions workflow was added under:

```text
.github/workflows/ci.yml
```

The CI workflow runs when code is pushed to `main` or when a pull request targets `main`.

The pipeline performs:

```text
checkout source code
        |
        v
configure Go
        |
        v
check formatting
        |
        v
go vet
        |
        v
run tests
        |
        v
build Go application
        |
        v
build Docker image
```

This means every code change must successfully build and pass tests before being considered valid.

---

# Step 9: GitHub Container Registry

Initially, Docker images existed only on the local machine.

That is not enough for a real deployment pipeline because another Kubernetes cluster would not have access to those images.

The pipeline was therefore configured to push images to GitHub Container Registry.

GHCR stands for:

```text
GitHub Container Registry
```

The image repository is:

```text
ghcr.io/bu3lii/cicd-go-k8s
```

The CI pipeline pushes images using tags such as:

```text
latest
```

and, more importantly:

```text
<Git commit SHA>
```

For example:

```text
ghcr.io/bu3lii/cicd-go-k8s:a14f930...
```

Using the Git commit SHA makes deployments traceable.

If Kubernetes is running:

```text
ghcr.io/bu3lii/cicd-go-k8s:a14f930...
```

we can identify exactly which Git commit produced that application version.

---

# Why not only use `latest`?

A tag such as:

```text
latest
```

can point to different images at different times.

That makes it difficult to know exactly what is deployed.

A commit SHA is immutable from the perspective of our build process.

For example:

```text
Git commit:
a14f930

Docker image:
ghcr.io/bu3lii/cicd-go-k8s:a14f930

Kubernetes:
ghcr.io/bu3lii/cicd-go-k8s:a14f930
```

Everything is linked together.

This makes debugging and rollback much easier.

---

# Step 10: Argo CD

Initially, Helm deployments were performed manually using commands such as:

```bash
helm upgrade
```

We then introduced Argo CD.

Argo CD runs inside Kubernetes and continuously watches Git.

The Argo CD Application points to:

```text
repository:
https://github.com/bu3lii/cicd-go-k8s

path:
helm/go-api
```

Argo CD treats Git as the desired state.

The new deployment model became:

```text
Git repository
      |
      v
Argo CD
      |
      v
Helm chart
      |
      v
Kubernetes
```

This is a GitOps approach.

Instead of manually telling Kubernetes:

> deploy version X

we update Git so it says:

> the desired version is X

Argo CD notices the difference and updates Kubernetes.

---

# Self-Healing with Argo CD

Argo CD was configured with:

```yaml
automated:
  prune: true
  selfHeal: true
```

`selfHeal` means Argo CD can detect some manual changes to Kubernetes and restore the state defined in Git.

`prune` means that if a Kubernetes resource is removed from Git, Argo CD can remove that resource from the cluster as well.

This makes Git the source of truth.

---

# Step 11: Connecting CI and CD

The final important piece was connecting GitHub Actions to Argo CD.

After GitHub Actions builds an image, it pushes:

```text
ghcr.io/bu3lii/cicd-go-k8s:<github.sha>
```

Then the workflow updates the Helm value:

```yaml
image:
  tag: <github.sha>
```

The workflow commits this change back into Git.

That creates the CD flow:

```text
application code changes
        |
        v
git push
        |
        v
GitHub Actions
        |
        v
tests pass
        |
        v
Docker image built
        |
        v
image pushed to GHCR
        |
        v
Helm image tag updated
        |
        v
change committed to Git
        |
        v
Argo CD detects change
        |
        v
Kubernetes rolling deployment
```

This is the complete CI/CD pipeline.

---

# Why the deployment commit uses `[skip ci]`

The GitHub Actions workflow makes its own commit when it updates the Helm image tag.

Without protection, this would happen:

```text
developer push
      |
      v
CI starts
      |
      v
CI updates Helm tag
      |
      v
CI creates Git commit
      |
      v
that commit starts CI again
      |
      v
CI creates another commit
      |
      v
loop
```

The automated deployment commit therefore contains:

```text
[skip ci]
```

This prevents the deployment-only commit from launching another full CI run.

Argo CD still sees the Git change and deploys it.

---

# Step 12: Trivy Security Scanning

Trivy was added to improve the CI pipeline.

Trivy scans the container image for known package vulnerabilities.

The intended pipeline becomes:

```text
build image
    |
    v
Trivy scan
    |
    +---- serious vulnerabilities ----> fail CI
    |
    v
push image
```

This means an image with vulnerabilities matching the configured severity policy can be prevented from reaching the deployment stage.

---

# Final CI/CD Flow

The final architecture can be summarized as:

```text
                    DEVELOPER
                        |
                        | git push
                        v
                 GITHUB REPOSITORY
                        |
                        v
                 GITHUB ACTIONS
                 /      |       \
                /       |        \
             tests    build     Trivy
                        |
                        v
                 DOCKER IMAGE
                        |
                        v
                       GHCR
                        |
                        |
            image tagged with Git SHA
                        |
                        v
           update Helm values.yaml
                        |
                        v
                 Git bot commit
                        |
                        v
                     ARGO CD
                        |
                        v
                       HELM
                        |
                        v
                   KUBERNETES
                        |
                 +------+------+
                 |             |
               POD 1          POD 2
                 \             /
                  \           /
                   K8S SERVICE
```

---

# Useful Commands

Check Kubernetes pods:

```bash
kubectl get pods
```

Watch a deployment happen:

```bash
kubectl get pods -w
```

Check the Deployment:

```bash
kubectl get deployment
```

Check Services:

```bash
kubectl get svc
```

Check the exact image Kubernetes is running:

```bash
kubectl get deployment go-api \
  -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
```

Check Argo CD:

```bash
kubectl get applications -n argocd
```

Expected state:

```text
Synced
Healthy
```

Port-forward the application:

```bash
kubectl port-forward service/go-api 8080:80
```

Then test:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/api/hello
```

---

# Technologies Used

* Go
* Docker
* Kubernetes
* Colima
* Helm
* GitHub
* GitHub Actions
* GitHub Container Registry
* Argo CD
* Trivy

---

# What This Project Demonstrates

This project demonstrates several DevOps concepts in one workflow:

### Continuous Integration

Every code change is automatically tested and built.

### Containerization

The Go application is packaged into a Docker image.

### Container Registry

Built images are stored remotely in GHCR.

### Kubernetes

The application runs as replicated containers managed by Kubernetes.

### Health Checking

Kubernetes uses readiness and liveness probes to determine application health.

### Self-Healing

Kubernetes recreates failed or deleted pods.

### Rolling Deployments

New versions can be introduced without stopping every running application instance at once.

### Helm

Kubernetes resources are packaged and parameterized.

### GitOps

Git contains the desired deployment state.

### Argo CD

Argo CD continuously reconciles Kubernetes with Git.

### Immutable Versioning

Deployments use Git SHA image tags rather than relying only on `latest`.

### Security Scanning

Container images are checked for known vulnerabilities before deployment.

---

# What I Learned

The biggest lesson from this project is that CI/CD is not just one tool.

Each tool has a specific responsibility.

```text
Go
→ application

Docker
→ package the application

GitHub Actions
→ build and test automation

GHCR
→ store container images

Helm
→ package Kubernetes configuration

Git
→ desired deployment configuration

Argo CD
→ synchronize Git with Kubernetes

Kubernetes
→ run and maintain the application
```

The combination of these tools creates the complete delivery system.

A developer only needs to push code.

The rest of the process can happen automatically.
