# Cloud Seminar Demo

A small Go web application for a two-hour cloud seminar. Students submit a form. The application stores each submission as a JSON object in Amazon S3. Kubernetes runs the application as a stateless container.

The seminar uses this path:

```
Browser
   ↓
Kubernetes Service (NodePort)
   ↓
Go application Pod(s)
   ↓
AWS SDK for Go v2
   ↓
Amazon S3 (prasad-cloud-demo)
```

Compute (Pods) can be restarted, scaled, and replaced. The form data stays in S3.

## What the application demonstrates

1. A simple web application written in Go.
2. Docker containerization.
3. Image distribution through Docker Hub (`joshiprasad/cloud-seminar-demo`).
4. Deployment to K3s with Helm.
5. A Kubernetes Service in front of the Pods.
6. A Deployment and its desired state (`replicaCount`).
7. Self-healing after the process exits (`GET /crash`).
8. Liveness (`GET /health`) and readiness (`GET /ready`) probes.
9. Stateless application compute.
10. Persistent data stored in Amazon S3, outside the Pod.
11. Scaling from one Pod to two Pods.
12. A rolling update by changing the image tag with Helm.
13. The difference between compute and object storage.

## Architecture

| Piece | Role |
| --- | --- |
| Go HTTP server | Renders the form and writes one JSON object per submission |
| Docker image | Packages the server. The final image has no Go compiler |
| Helm chart `cloud-demo` | Creates the Deployment, Service, ConfigMap, and ServiceAccount |
| Kubernetes Service | Stable address in front of one or more Pods |
| Amazon S3 bucket `prasad-cloud-demo` | Persistent storage for submissions |
| EC2 IAM role | Credentials for S3. The application does not contain access keys |

The cluster is a single-node K3s install on an EC2 VM in `us-west-2a`. The application region is `us-west-2`.

There is no PersistentVolume and no S3 mount. The Pod filesystem is not the source of truth.

## Repository structure

```
cloud-seminar-demo/
├── app/
│   ├── go.mod
│   ├── main.go            server startup and graceful shutdown
│   ├── config.go          environment variables
│   ├── handlers.go        HTTP endpoints
│   ├── submission.go      validation, ids, object keys, JSON
│   ├── store.go           S3 writer
│   └── templates/         server-rendered HTML
├── Dockerfile
├── helm/cloud-demo/       K3s chart
└── docs/seminar-demo.md   step-by-step teaching sequence
```

## Configuration

The process reads these environment variables. Empty or unset variables use the defaults.

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_NAME` | `cloud-seminar-demo` | Shown on the page and on `GET /info` |
| `APP_VERSION` | `1.0.0` | Shown on `GET /info` |
| `PORT` | `8080` | HTTP listen port |
| `AWS_REGION` | `us-west-2` | Region passed to the AWS SDK |
| `S3_BUCKET` | `prasad-cloud-demo` | Bucket that receives submissions |

Helm maps the same values from `helm/cloud-demo/values.yaml` into a ConfigMap.

AWS credentials are not configuration in this project. The AWS SDK for Go v2 uses its default credential chain:

1. Environment variables, if they already exist on the machine.
2. The shared AWS config file (`~/.aws`), if one exists.
3. The EC2 instance IAM role, when the process runs on the seminar VM.

Do not commit access keys, secret keys, or `.env` files. The `/info` page prints only the fields in the table above, plus the hostname and the storage backend (`s3`).

## Local Go execution

Run these commands on the developer machine.

Requires Go 1.24 or newer. The AWS SDK module asks for that minimum. An older `go` command will download a newer toolchain on its own when it can reach the internet.

```bash
cd app
go test ./...
go run .
```

The server listens on port 8080. Open `http://127.0.0.1:8080/`.

`GET /health`, `GET /ready`, and `GET /info` work without AWS. A form submission calls S3. On a laptop that already has an AWS profile with `s3:PutObject` on `prasad-cloud-demo`, the object is created. Without credentials, the browser shows a short error page and the process keeps running. The technical error is written to the log only.

Stop the process with Ctrl+C. It handles `SIGINT` and `SIGTERM` and shuts down within 10 seconds.

Override a setting for one run:

```bash
cd app
S3_BUCKET=prasad-cloud-demo AWS_REGION=us-west-2 PORT=8080 go run .
```

## Build the Docker image

Run this on the developer machine, from the repository root.

```bash
docker build -t joshiprasad/cloud-seminar-demo:1.0.0 .
```

The Dockerfile is multi-stage. The first stage compiles a static binary. The final stage is `gcr.io/distroless/static-debian12:nonroot`. It contains the binary, listens on port 8080, and runs as uid 65532. It has no shell.

## Run the Docker container

Run this on the developer machine.

```bash
docker run --rm -p 8080:8080 joshiprasad/cloud-seminar-demo:1.0.0
```

On a laptop, the container uses the same credential chain as `go run`. The seminar VM does not need a credentials file. K3s runs the container with the EC2 instance IAM role. Do not pass access keys on the `docker run` command line and do not bake them into the image.

Open `http://127.0.0.1:8080/`. Stop the container with Ctrl+C.

## Push the image to Docker Hub

Run this on the developer machine. Log in once if `docker push` asks for credentials.

```bash
docker login
docker push joshiprasad/cloud-seminar-demo:1.0.0
```

K3s pulls this image itself. A `docker tag` on the VM does not place the image in the K3s container store. Push to Docker Hub, then let the node pull `joshiprasad/cloud-seminar-demo:1.0.0`.

The seminar flow expects that repository to be public, so the node can pull it without a registry secret. If the repository is private, create a pull secret and install with `--set imagePullSecrets[0].name=regcred`.

## Install the Helm chart on K3s

Run the Helm and kubectl commands on the AWS EC2 VM (`us-west-2a`), not on the laptop.

Prerequisites on that VM:

- K3s is installed and the node is Ready.
- Helm 3 is installed.
- `kubectl` works. If it does not, use `sudo k3s kubectl` in place of `kubectl`, or copy the kubeconfig:

```bash
mkdir -p ~/.kube
sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
sudo chown "$USER" ~/.kube/config
export KUBECONFIG=$HOME/.kube/config
```

- The instance IAM role can `s3:PutObject` on `arn:aws:s3:::prasad-cloud-demo/submissions/*`.
- The instance metadata hop limit is at least 2, so a Pod can reach the instance role. The default hop limit of 1 is visible on the VM itself and is not visible from a Pod:

```bash
aws ec2 modify-instance-metadata-options \
  --instance-id <instance-id> \
  --http-endpoint enabled \
  --http-put-response-hop-limit 2 \
  --region us-west-2
```
- The security group allows the NodePort from the network you will use in the browser (ports 30000–32767, or the single port Kubernetes assigns).
- This repository is present on the VM (`git clone` or copy). The chart has to be on the machine where you run Helm. The running container comes from Docker Hub.

```bash
git clone https://github.com/joshi-prasad/cloud-seminar-demo.git
cd cloud-seminar-demo
helm install cloud-demo ./helm/cloud-demo
```

`helm install` creates:

- Deployment `cloud-demo` (1 replica)
- Service `cloud-demo` (NodePort, port 8080)
- ConfigMap `cloud-demo` (`APP_NAME`, `APP_VERSION`, `PORT`, `AWS_REGION`, `S3_BUCKET`)
- ServiceAccount `cloud-demo` (no IAM annotation)

Check that the Pod becomes Ready:

```bash
kubectl rollout status deployment/cloud-demo
kubectl get pods,svc,deploy,configmap,serviceaccount -l app.kubernetes.io/instance=cloud-demo
```

## Upgrade the Helm release

Run this on the EC2 VM, from the repository directory.

```bash
helm upgrade cloud-demo ./helm/cloud-demo --reuse-values
```

`--reuse-values` keeps settings from the previous install and applies changes you pass with `--set` or a new values file.

A version rollout used in the seminar:

```bash
helm upgrade cloud-demo ./helm/cloud-demo \
  --reuse-values \
  --set image.tag=1.1.0 \
  --set application.version=1.1.0
kubectl rollout status deployment/cloud-demo
```

The Deployment uses a rolling update with `maxUnavailable: 0` and `maxSurge: 1`. Kubernetes starts a Pod with the new image, waits until it is Ready, then removes the old Pod. The image `joshiprasad/cloud-seminar-demo:1.1.0` must already exist in Docker Hub. For the seminar, that tag can be the same binary retagged and pushed; `APP_VERSION` is what `GET /info` displays.

```bash
docker tag joshiprasad/cloud-seminar-demo:1.0.0 joshiprasad/cloud-seminar-demo:1.1.0
docker push joshiprasad/cloud-seminar-demo:1.1.0
```

Run the tag and push on the developer machine.

## Change replicaCount

Run this on the EC2 VM.

```bash
helm upgrade cloud-demo ./helm/cloud-demo --reuse-values --set replicaCount=2
kubectl get pods -l app.kubernetes.io/instance=cloud-demo
```

Return to one Pod with `--set replicaCount=1`. The application does not change. Each Pod has its own hostname on `GET /info`. Submissions from either Pod go to the same S3 bucket.

## Find the NodePort

Run this on the EC2 VM.

```bash
kubectl get svc cloud-demo
export NODE_PORT=$(kubectl get svc cloud-demo -o jsonpath='{.spec.ports[0].nodePort}')
echo "$NODE_PORT"
```

The service line looks like `8080:3xxxx/TCP`. `3xxxx` is the NodePort.

## Open the application in a browser

On the EC2 VM itself:

```bash
curl -sS "http://127.0.0.1:${NODE_PORT}/"
```

From a laptop browser, use the EC2 public IP:

```text
http://<ec2-public-ip>:<nodePort>/
```

Find the public IP in the AWS console, or on the VM:

```bash
curl -sS http://169.254.169.254/latest/meta-data/public-ipv4
```

The security group must allow inbound TCP on that NodePort. The form has Name, Email, and Message. After a successful submit, the page shows the S3 object key. The key looks like:

```text
submissions/2026-09-29T10-45-12Z-a83f21.json
```

## Test /health

Liveness probe. HTTP 200 and the body `ok` means the process is alive.

On the EC2 VM:

```bash
curl -sS -D - "http://127.0.0.1:${NODE_PORT}/health"
```

## Test /ready

Readiness probe. HTTP 200 and the body `ready` means the process can serve requests and its storage client is configured. This check does not call S3 on every probe. A short S3 outage should not remove every Pod from the Service.

```bash
curl -sS -D - "http://127.0.0.1:${NODE_PORT}/ready"
```

## Test /info

```bash
curl -sS "http://127.0.0.1:${NODE_PORT}/info"
```

Example:

```text
name: cloud-seminar-demo
version: 1.0.0
hostname: cloud-demo-xxxxxxxxxx-xxxxx
storage: s3
region: us-west-2
bucket: prasad-cloud-demo
```

With two replicas, refresh a few times. The Service can send you to either Pod, so the hostname can change. The page does not print environment variables, request headers, or credentials.

## Trigger /crash

`GET /crash` exits the process with status 1. Kubernetes then starts a new container in the same Pod.

On the EC2 VM, in one terminal:

```bash
kubectl get pods -l app.kubernetes.io/instance=cloud-demo -w
```

In another terminal:

```bash
curl -sS "http://127.0.0.1:${NODE_PORT}/crash" || true
```

`curl` fails because the process exits before it can send a normal response. That is the point of the endpoint.

## Watch Pod restarts

Leave `kubectl get pods -w` running across the crash. The Pod stays, `READY` drops, `RESTARTS` increases, then the Pod becomes `1/1 Running` again.

After the new container is up, the previous container's log is still available:

```bash
kubectl logs deploy/cloud-demo --previous
```

You should see `crash requested; exiting` and then, in the new container:

```bash
kubectl logs deploy/cloud-demo
```

a new `starting` line. The Deployment's desired replica count never changed. Kubernetes replaced the failed container.

## Inspect S3 objects

Run these with the AWS CLI anywhere your identity can read the bucket (your laptop or the EC2 VM).

```bash
aws s3 ls s3://prasad-cloud-demo/submissions/ --region us-west-2
aws s3 cp "s3://prasad-cloud-demo/submissions/<object-key>" - --region us-west-2
```

Use the object key from the confirmation page, including the `submissions/` prefix.

The object is JSON:

```json
{
  "id": "a83f21",
  "timestamp": "2026-09-29T10:45:12Z",
  "name": "Ada",
  "email": "ada@example.com",
  "message": "Hello from the seminar"
}
```

After `/crash` and a restart, list the bucket again. The object is still there. Deleting a Pod does not delete it.

The instructor identity that runs `aws s3 ls` needs `s3:ListBucket` and `s3:GetObject`. The application identity only needs `s3:PutObject`.

## Uninstall the Helm release

Run this on the EC2 VM.

```bash
helm uninstall cloud-demo
kubectl get pods,svc
```

The Pods and the Service are gone. Objects already written to `s3://prasad-cloud-demo/submissions/` remain. That is the compute versus storage point of the seminar.

## Endpoints

| Method | Path | Behavior |
| --- | --- | --- |
| `GET` | `/` | HTML form |
| `POST` | `/submit` | Validates the form and stores one JSON object in S3 |
| `GET` | `/health` | `200 ok` when the process is alive |
| `GET` | `/ready` | `200 ready` when the server can accept work |
| `GET` | `/info` | Name, version, hostname, storage, region, bucket |
| `GET` | `/crash` | Exits with status 1 |

If S3 rejects a submission, the log contains the AWS error and the browser shows a short message. The page does not include the AWS error text.

## Troubleshooting

**`ImagePullBackOff` or `ErrImagePull`.** The tag is missing on Docker Hub, the repository is private, or the node cannot reach Docker Hub. Confirm `docker push joshiprasad/cloud-seminar-demo:1.0.0` succeeded and that the repository is public. Then `kubectl describe pod -l app.kubernetes.io/instance=cloud-demo`.

**Pod `CrashLoopBackOff` before you call `/crash`.** Read `kubectl logs deploy/cloud-demo`. Configuration errors (for example `PORT=0`) stop the process at startup. The chart's ConfigMap sets `PORT` to `8080`.

**The form returns "could not be saved".** Read the Pod log. Typical causes: the instance role cannot `s3:PutObject`, the bucket name does not match `prasad-cloud-demo`, the region is not `us-west-2`, or the metadata hop limit is still 1 so the Pod cannot see the instance role. The app is still serving `/health`. Fix IAM, the hop limit, or the ConfigMap value and upgrade the release. The Pod template carries a checksum of the ConfigMap, so a values change rolls the Pod.

**The browser cannot open the NodePort.** `curl` to `127.0.0.1:$NODE_PORT` on the VM works, but the laptop times out. Open the NodePort in the EC2 security group, and use the public IP. Also confirm you are not using port 8080 from the laptop. Port 8080 is the container port. The laptop uses the NodePort.

**`kubectl` cannot find the cluster.** Export `KUBECONFIG` as shown above, or call `sudo k3s kubectl`.

**Helm cannot be found on the VM.** Install Helm 3 on the EC2 VM. Building the image does not install Helm.

**`/info` still shows the old version after an upgrade.** `application.version` is independent of `image.tag`. Set both, as in the upgrade command above. The config checksum annotation restarts Pods when the ConfigMap changes.

**Local `go test` tries to use AWS.** It should not. Tests use an in-memory store and a fake S3 client. No credentials are required.

## Development checks

Run these on the developer machine before a seminar.

```bash
cd app
gofmt -w .
go mod tidy
go vet ./...
go test ./...
cd ..
helm lint helm/cloud-demo
helm template cloud-demo helm/cloud-demo
```

The teaching sequence with the commands in order is in [docs/seminar-demo.md](docs/seminar-demo.md).
