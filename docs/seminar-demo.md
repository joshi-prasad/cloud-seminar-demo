# Seminar demonstration runbook

Two-hour sequence for the Cloud Seminar Demo. Each step says where to run the command and what the students should take from it.

The application path is:

```
Browser
   ↓
Kubernetes Service
   ↓
Go application Pod(s)
   ↓
AWS SDK for Go v2
   ↓
Amazon S3 (prasad-cloud-demo)
```

## Before the session

**Developer machine**

- Go 1.24 or newer, Docker, and an AWS identity that can read `s3://prasad-cloud-demo` for the inspection step.
- Docker Hub login that can push to `joshiprasad/cloud-seminar-demo`.
- Repository cloned from `https://github.com/joshi-prasad/cloud-seminar-demo`.

**EC2 VM (K3s, us-west-2a)**

- K3s is up: `kubectl get nodes` shows the node `Ready`.
- Helm 3 is installed.
- The instance IAM role allows `s3:PutObject` on `arn:aws:s3:::prasad-cloud-demo/submissions/*`.
- Instance metadata hop limit is at least 2, so Pods can use that role. A hop limit of 1 works for processes on the VM and fails inside a Pod. Set it with `aws ec2 modify-instance-metadata-options --http-put-response-hop-limit 2`.
- The security group will allow the NodePort you are assigned (TCP 30000–32767 is the Kubernetes range).
- The repository is cloned on the VM so Helm can read `helm/cloud-demo`.
- Outbound access from the VM to Docker Hub works. The image must be public, or a pull secret must already exist.

Suggested pacing: steps A–D about 25 minutes, E–H about 25 minutes, I–Q about 30 minutes, R–W about 30 minutes.

---

## A. Show the Go application locally

**Where:** developer machine.

```bash
cd app
go test ./...
go run .
```

Open `http://127.0.0.1:8080/`. Show the form, then `http://127.0.0.1:8080/info`.

If your laptop can write to the bucket, submit the form once and leave the confirmation page up. If it cannot, show the error page and point at the log line. The process stays up either way.

Stop it with Ctrl+C and show the shutdown log line.

**Teaching point.** This is an ordinary Go program. It listens on a port, renders HTML, and uses the AWS SDK to put one JSON object in S3. Nothing here is Kubernetes yet. The HTML is rendered by the server. There is no database inside the process, and the process does not keep the submissions.

---

## B. Build the Docker image

**Where:** developer machine, repository root.

Stop the local `go run` process first so port 8080 is free.

```bash
docker build -t joshiprasad/cloud-seminar-demo:1.0.0 .
```

**Teaching point.** The Dockerfile has two stages. The Go compiler exists only in the build stage. The image that will run on the VM is a small runtime image with the binary, running as a non-root user. The tag is `1.0.0`, not `latest`, so a later upgrade can name a different tag.

---

## C. Run the container

**Where:** developer machine.

```bash
docker run --rm -p 8080:8080 joshiprasad/cloud-seminar-demo:1.0.0
```

Open `http://127.0.0.1:8080/health` and `http://127.0.0.1:8080/info`.

**Teaching point.** The same binary now runs inside a container. The laptop reaches it through a published port. The image has no shell, so the way to see what it is doing is its HTTP endpoints and its logs. Stop it with Ctrl+C when you are ready to move on.

---

## D. Push the image to Docker Hub

**Where:** developer machine.

```bash
docker login
docker push joshiprasad/cloud-seminar-demo:1.0.0
```

**Teaching point.** The VM does not receive the image from your laptop's Docker daemon. Docker Hub is the shared registry. K3s will pull `joshiprasad/cloud-seminar-demo:1.0.0` by that name.

---

## E. Deploy to K3s with Helm

**Where:** EC2 VM.

```bash
cd cloud-seminar-demo
helm install cloud-demo ./helm/cloud-demo
kubectl rollout status deployment/cloud-demo
```

**Teaching point.** Helm rendered the chart into a Deployment, a Service, a ConfigMap, and a ServiceAccount. The Deployment's desired state is one Pod running that image. You did not start the process by hand on the VM. The ConfigMap supplies `APP_NAME`, `APP_VERSION`, `PORT`, `AWS_REGION`, and `S3_BUCKET`. There is still no access key in the chart. The Pod uses the EC2 instance IAM role.

---

## F. Access the application through the Kubernetes Service

**Where:** EC2 VM, then the audience browser.

```bash
kubectl get svc cloud-demo
export NODE_PORT=$(kubectl get svc cloud-demo -o jsonpath='{.spec.ports[0].nodePort}')
echo "$NODE_PORT"
curl -sS "http://127.0.0.1:${NODE_PORT}/health"
```

In the browser:

```text
http://<ec2-public-ip>:<nodePort>/
```

Public IP, from the VM:

```bash
curl -sS http://169.254.169.254/latest/meta-data/public-ipv4
```

**Teaching point.** The Service is the stable address. Clients use the NodePort. They do not need the Pod IP, and Pod IPs change. Port 8080 is the port inside the container. The NodePort is the port on the VM.

---

## G. Submit a form

**Where:** browser, against the NodePort URL.

Enter a name, an email, and a message. Submit.

The confirmation page shows an object key like `submissions/2026-09-29T10-45-12Z-a83f21.json`.

**Teaching point.** The request arrived at the Service, then at the Pod. The Pod generated a unique key and wrote JSON to S3. The confirmation page is not reading the message back from disk on the Pod. It is showing the key that was just written.

---

## H. Show the S3 object

**Where:** a machine with the AWS CLI that can read the bucket (developer machine or the EC2 VM).

```bash
aws s3 ls s3://prasad-cloud-demo/submissions/ --region us-west-2
aws s3 cp "s3://prasad-cloud-demo/submissions/<object-key>" - --region us-west-2
```

Paste the key from the confirmation page.

**Teaching point.** The persistent copy lives in the bucket `prasad-cloud-demo` in `us-west-2`. The object is independent of the Pod that wrote it.

---

## I. Show the Pods

**Where:** EC2 VM.

```bash
kubectl get pods -l app.kubernetes.io/instance=cloud-demo -o wide
```

**Teaching point.** A Pod is the running copy of the container. With `replicaCount: 1` there is one. Note the Pod name and the node. The node is this EC2 VM.

---

## J. Show the Deployment

**Where:** EC2 VM.

```bash
kubectl get deployment cloud-demo
```

**Teaching point.** The Deployment records the desired state: one replica, the image `joshiprasad/cloud-seminar-demo:1.0.0`. The Pod is how that desire is currently met. If reality drifts, Kubernetes tries to match the Deployment again.

---

## K. Show the Service

**Where:** EC2 VM.

```bash
kubectl get service cloud-demo
```

**Teaching point.** The Service selects Pods with the labels from the chart (`app.kubernetes.io/instance=cloud-demo`). Adding a second replica later does not require a new Service. The NodePort stays the same.

---

## L. Describe a Pod

**Where:** EC2 VM.

```bash
kubectl get pods -l app.kubernetes.io/instance=cloud-demo
kubectl describe pod <pod-name>
```

Point at the image, the container port 8080, the environment from the ConfigMap, the liveness probe `GET /health`, the readiness probe `GET /ready`, and the non-root user.

**Teaching point.** The probe and resource settings came from the Helm chart, not from a manual `docker run`. The Pod is not mounting a volume for the form data.

---

## M. Demonstrate /health, /ready, and /info

**Where:** EC2 VM.

```bash
curl -sS "http://127.0.0.1:${NODE_PORT}/health"
curl -sS "http://127.0.0.1:${NODE_PORT}/ready"
curl -sS "http://127.0.0.1:${NODE_PORT}/info"
```

**Teaching point.** `/health` answers "is the process alive?". `/ready` answers "should the Service send it traffic?". `/info` shows the hostname of whichever Pod answered, plus the configured region and bucket. It does not show credentials. Readiness does not call S3 every few seconds, so a storage blip does not take the Pod out of the Service.

---

## N. Demonstrate failure with /crash

**Where:** EC2 VM.

Start the watch in a second terminal before you call crash (step O). Then:

```bash
curl -sS "http://127.0.0.1:${NODE_PORT}/crash" || true
```

**Teaching point.** `/crash` is a deliberate `exit 1`. The container stops. This is the failure you are about to watch Kubernetes repair. You did not delete the Deployment.

---

## O. Watch the Pods

**Where:** EC2 VM, started just before step N.

```bash
kubectl get pods -l app.kubernetes.io/instance=cloud-demo -w
```

Wait until the Pod is `1/1 Running` again and `RESTARTS` is at least 1.

```bash
kubectl logs deploy/cloud-demo --previous
kubectl logs deploy/cloud-demo
```

**Teaching point.** `--previous` is the container that exited. Its log contains `crash requested; exiting`. The new log contains `starting` and `listening`. Same Pod name, new container.

---

## P. Explain the restart

Say this while the watch output is still visible:

```
process/container failure
    ↓
Kubernetes detects failure
    ↓
container restarts
    ↓
application becomes Ready again
```

**Teaching point.** The kubelet noticed the container had exited. The restart policy on the Pod starts a new container. The liveness probe is what would catch a process that is still running but no longer healthy. An immediate exit is noticed without waiting for the probe period. Desired state stayed at one replica the whole time.

---

## Q. Show that the S3 object is still there

**Where:** AWS CLI.

```bash
aws s3 ls s3://prasad-cloud-demo/submissions/ --region us-west-2
```

**Teaching point.** Restarting the container did not delete the submission. The Pod was compute. S3 is the storage. The new container can accept another submission into the same bucket.

---

## R. Scale to two Pods

**Where:** EC2 VM, from the repository directory.

```bash
helm upgrade cloud-demo ./helm/cloud-demo --reuse-values --set replicaCount=2
kubectl rollout status deployment/cloud-demo
```

**Teaching point.** You changed the desired replica count through Helm. You did not rebuild the application. The Service now has two endpoints. `--reuse-values` keeps the image tag and the bucket name from the install.

---

## S. Show two Pods

**Where:** EC2 VM and the browser.

```bash
kubectl get pods -l app.kubernetes.io/instance=cloud-demo -o wide
```

Refresh `http://<ec2-public-ip>:<nodePort>/info` several times.

**Teaching point.** Two hostnames mean two Pods. The Service is choosing a Pod for each request. Either Pod can store a submission, because neither Pod owns the data. Submit once more if you want a second object in the bucket.

---

## T. Delete one Pod

**Where:** EC2 VM.

```bash
kubectl get pods -l app.kubernetes.io/instance=cloud-demo
kubectl delete pod <one-pod-name>
```

**Teaching point.** Deleting a Pod is another kind of failure. You removed a running copy. You did not change the Deployment's desired count, which is still 2.

---

## U. Show that the missing Pod comes back

**Where:** EC2 VM.

```bash
kubectl get pods -l app.kubernetes.io/instance=cloud-demo -w
```

**Teaching point.** The Deployment creates a replacement so the count matches `replicaCount`. The new Pod has a new name and a new hostname. The Service starts sending it traffic after the readiness probe succeeds. The other Pod kept serving while this happened.

---

## V. Compute versus storage

Say this after the replacement Pod is Ready:

```
Kubernetes manages compute lifecycle.
S3 stores persistent application data.
```

List the bucket once more:

```bash
aws s3 ls s3://prasad-cloud-demo/submissions/ --region us-west-2
```

**Teaching point.** Pods were crashed, scaled, and deleted. The JSON objects are still in `prasad-cloud-demo`. Uninstalling the release later removes the Pods and the Service and still leaves the objects.

---

## W. Image version upgrade

Do this when `joshiprasad/cloud-seminar-demo:1.1.0` exists. It can be the same binary with a new tag.

**Where:** developer machine.

```bash
docker tag joshiprasad/cloud-seminar-demo:1.0.0 joshiprasad/cloud-seminar-demo:1.1.0
docker push joshiprasad/cloud-seminar-demo:1.1.0
```

**Where:** EC2 VM.

```bash
helm upgrade cloud-demo ./helm/cloud-demo \
  --reuse-values \
  --set image.tag=1.1.0 \
  --set application.version=1.1.0
kubectl rollout status deployment/cloud-demo
kubectl get pods -l app.kubernetes.io/instance=cloud-demo -w
curl -sS "http://127.0.0.1:${NODE_PORT}/info"
```

**Teaching point.** Changing `image.tag` starts a rolling update. The chart sets `maxUnavailable: 0` and `maxSurge: 1`, so a new Pod becomes Ready before an old one is removed. With two replicas, one Pod keeps serving during the rollout. `/info` shows `version: 1.1.0` because `APP_VERSION` came from the ConfigMap. The bucket did not move, and earlier objects are still there.

```bash
kubectl rollout history deployment/cloud-demo
```

---

## Close the session

**Where:** EC2 VM, when you want the demo resources removed.

```bash
helm uninstall cloud-demo
aws s3 ls s3://prasad-cloud-demo/submissions/ --region us-west-2
```

The Kubernetes objects are gone. The S3 objects remain until someone deletes them with the AWS CLI.
