# Prism Central RBAC and authentication for the Nutanix COSI driver

## Recommended path: PC Service Account + API key

The driver supports two authentication modes against the PC IAM proxy:

| Mode | How to configure | HTTP header sent on every PC call |
| --- | --- | --- |
| **Service Account API key** (recommended) | `secret.pc_api_key` / `--pc_api_key` / env `PC_API_KEY` | `X-ntnx-api-key: <api_key>` |
| HTTP Basic Auth (legacy) | `secret.pc_username` + `secret.pc_password` (or `--pc_secret <user>:<pass>` / env `PC_SECRET`) | `Authorization: Basic base64(user:pass)` |

The selection rule is a single rule applied everywhere:

- If `PC_API_KEY` is non-empty -> the driver attaches the `X-ntnx-api-key` header.
- Else if `PC_SECRET` (`user:pass`) is non-empty -> the driver attaches `Authorization: Basic ...`.
- Else -> the driver refuses to start.

The Service Account path is preferred because:

- The credential is scoped to one specific identity that can be revoked or rotated without touching any human PC user.
- The role bound to the Service Account narrows the blast radius to exactly what the driver needs (see "Minimum PC role" below).
- Basic Auth ties the driver to the lifecycle of a human PC user; rotating that user's password forces a driver restart and risks accidentally locking the driver out.

Existing installs that only configure `pc_username` / `pc_password` continue to work unchanged.

## PC API calls the driver makes

All calls go to the IAM proxy hosted on the Prism Central endpoint configured via `PC_ENDPOINT`. The driver authenticates each of these calls using the rule above.

| Call | Method + Path | Triggered by |
| --- | --- | --- |
| Create IAM user + access key | `POST /oss/iam_proxy/buckets_access_keys` | `BucketAccess` grant (a new application requesting access to a bucket) |
| List IAM users | `GET /oss/iam_proxy/users` | Fallback when the create call reports the user already exists; the driver looks the user up by name to re-mint a key |
| Mint a new access key for an existing user | `POST /oss/iam_proxy/users/{uuid}/buckets_access_keys` | Same fallback path as above |
| Delete IAM user | `DELETE /oss/iam_proxy/users/{uuid}` | `BucketAccess` revoke (application releasing access) |

In addition, the driver issues S3 calls directly against the **Object Store** endpoint (`ENDPOINT`, separate from `PC_ENDPOINT`) using the admin S3 access/secret key pair:

- `CreateBucket` -- `BucketClaim` provision.
- `DeleteBucket` -- `BucketClaim` delete.
- `GetBucketPolicy` / `PutBucketPolicy` -- attach or detach an IAM user to a bucket on `BucketAccess` grant/revoke.

These S3 calls do **not** go through PC and are not gated by the Service Account; they are authenticated by the S3 admin keys (`ACCESS_KEY` / `SECRET_KEY` in the driver's secret).

## Minimum PC role

The Service Account must hold a role on the target Objects instance that grants, at minimum, the following capabilities:

- **View/Create/Delete/Update Object Store** -- Needed to create and delete IAM users and to mint access keys.
- **Edit Buckets Object Store** -- needed to create / delete buckets and to get / put bucket policies.

The simplest fit is the built-in **"Object Admin"** role scoped to the target Objects instance. Sites that want to narrow the surface further can build a custom role with just the capabilities listed above; the IAM-user and bucket capabilities are the ones the driver actually exercises.

The role must be assigned to the Service Account on the target Objects instance (or a category that includes it), not globally on PC.

## How to create the Service Account in Prism Central

The exact UI path may differ slightly between PC versions, but the high-level steps are the same:

1. Open Prism Central -> **Admin Center**.
2. Navigate to **Identity Providers** / **Service Accounts** and choose **Create Service Account**.
3. Give it a descriptive name (e.g. `cosi-driver`) and create it.
4. Generate an **API key** for the new Service Account and copy it somewhere safe -- PC will not show the secret again.
5. Assign the Service Account a role (e.g. **Object Admin**) on the target Objects instance, scoped tightly to that instance.

## How to configure the driver

### Helm `--set` flag

```console
helm install cosi-driver -n cosi-driver-nutanix . \
  --set secret.endpoint=<object-store-endpoint> \
  --set secret.access_key=<admin-access-key> \
  --set secret.secret_key=<admin-secret-key> \
  --set secret.pc_endpoint=<pc-endpoint> \
  --set secret.pc_api_key=<service-account-api-key>
```

When `secret.pc_api_key` is set the chart renders `PC_API_KEY` into the driver's secret and stops requiring `pc_username` / `pc_password`. Existing installs that still use the user/password pair continue to work.

### `values.yaml` snippet

```yaml
secret:
  enabled: true
  endpoint: "http://10.51.142.82:80"
  access_key: "<admin-access-key>"
  secret_key: "<admin-secret-key>"
  pc_endpoint: "https://10.51.149.82:9440"
  pc_api_key: "<service-account-api-key>"
```

### Plain Kubernetes secret (non-Helm)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: objectstorage-provisioner
type: Opaque
stringData:
  ENDPOINT: "http://10.51.142.82:80"
  ACCESS_KEY: "<admin-access-key>"
  SECRET_KEY: "<admin-secret-key>"
  PC_ENDPOINT: "https://10.51.149.82:9440"
  PC_API_KEY: "<service-account-api-key>"
```

After applying the secret, restart the `objectstorage-provisioner` pod so the new credentials are picked up.
