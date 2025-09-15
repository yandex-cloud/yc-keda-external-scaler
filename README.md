# yc-keda-external-scaler
[KEDA External Scaler](https://keda.sh/docs/2.17/concepts/external-scalers/) for [Yandex Cloud Monitoring](https://yandex.cloud/en/services/monitoring).

## Deployment

1. [Install](https://keda.sh/docs/2.17/deploy/) KEDA.
2. [Create](https://yandex.cloud/en/docs/iam/operations/sa/create) a service account with the `monitoring.viewer` role.
3. [Create](https://yandex.cloud/en/docs/iam/operations/authentication/manage-authorized-keys#create-authorized-key) an authorized key and save it locally.

```bash
git clone https://github.com/yandex-cloud/yc-keda-external-scaler.git
helm install yc-keda-external-scaler yc-keda-external-scaler/helm/yc-keda-external-scaler/. --set-file secret.data=./key.json
```

## Usage with ScaledObject

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: keda-demo-scaler
spec:
  scaleTargetRef:
    name: my-demo-app
  pollingInterval: 30
  cooldownPeriod: 300
  minReplicaCount: 1
  maxReplicaCount: 10
  triggers:
  - type: external
    metadata:
      scalerAddress: yc-keda-external-scaler.default.svc.cluster.local:8080
      # Yandex Monitoring query
      query: | 
        series_sum("load_balancer.requests_count_per_second"{service="application-load-balancer", load_balancer="xxx", http_router="*", virtual_host="*", route="route-xxx", backend_group="*", backend="*", zone="*", code="total"})
      # Yandex folder ID containing the monitoring metrics
      folderId: "xxx"
      # Target metric value per pod
      targetValue: "100"
      # Downsampling configuration
      downsampling.gridAggregation: "AVG"
      downsampling.gridInterval: "120000" # ms, aggregate metrics in a 2-minute window to get one average value
      # Time windows configuration
      timeWindow: "2m"          # Analyze 2 minutes of data
      timeWindowOffset: "30s"    # Offset 30 seconds back
```

## ScaledObject Metadata

### Server-Side Options

| Field | Description | Default | Options |
|-------|-------------|---------|---------|
| `query` | [Yandex Monitoring query](https://yandex.cloud/en/docs/monitoring/concepts/querying) | **Required** | - |
| `folderId` | [Yandex Cloud folder ID](https://yandex.cloud/en/docs/resource-manager/operations/folder/get-id)  | **Required** | - |
| `targetValue` | Target metric value for scaling | **Required** | Any positive number |
| `timeWindow` | Time range for metric query | `5m` | Go duration format: `1m`, `2m30s`, `5m` |
| `timeWindowOffset` | Offset to shift time window back (avoids trailing zeros) | `30s` | Go duration format: `30s`, `1m`, `2m` |
| `downsampling.gridAggregation` | Yandex Monitoring downsampling aggregation function | - | `MAX`, `MIN`, `SUM`, `AVG`, `LAST`, `COUNT` |
| `downsampling.gapFilling` | Yandex Monitoring parameters for filling in missing data | - | `NULL`, `NONE`, `PREVIOUS` |
| `downsampling.maxPoints` | Yandex Monitoring maximum number of points per request | - | Integer >= 10 (mutually exclusive) |
| `downsampling.gridInterval` | Yandex Monitoring time window for downsampling | - | Integer > 0 (mutually exclusive) |
| `downsampling.disabled` | Yandex Monitoring disable downsampling (get raw data) | - | `true`, `false` (mutually exclusive) |

> Important: Don't include `folderId` in the query body – `folderId` should be provided in the HTTP request as a [query parameter](https://yandex.cloud/en/docs/monitoring/concepts/querying#selectors) (as this ExternalScaler does).

Yandex Monitoring API supports server-side [downsampling](https://yandex.cloud/en/docs/monitoring/concepts/decimation) to reduce data transfer and improve performance. **If no downsampling options are specified, the Yandex Monitoring will use its default settings.**

The `timeWindow` field controls how far back in time to query metrics. This directly affects scaling responsiveness:
- **Shorter windows** (e.g., `1m`, `2m`): Faster response to traffic changes, but may be more sensitive to noise
- **Longer windows** (e.g., `5m`, `10m`): More stable scaling decisions, but slower response to traffic changes

**Format**: Use Go duration format - `30s`, `1m`, `2m30s`, `5m`, etc.

The `timeWindowOffset` parameter shifts the entire query time window backwards to avoid querying data that hasn't been fully ingested yet. This helps eliminate trailing zero values.

### Scaler-Side Options

| Field | Description | Default | Options |
|-------|-------------|---------|---------|
| `nanStrategy` | How to handle NaN values (client-side) | `error` | `skip`, `zero`, `error`, `lastValid` |
| `aggregationMethod` | How to aggregate multiple metrics (client-side) | `max` | `sum`, `avg`, `max`, `min`, `last` |
| `timeSeriesAggregation` | How to aggregate time series data (client-side) | None | `sum`, `avg`, `max`, `min`, `last` |

`nanStrategy` field allows to handle NaN values at the scaler side. 
- **`error`** (default): Return error if values are NaN
- **`skip`**: Ignore NaN values in calculations
- **`zero`**: Convert NaN to 0 (useful for counters like RPS)
- **`lastValid`**: Use the last valid value when NaN is encountered

```
Raw data: [10.5, "NaN", 15.2, "NaN", 20.1]

skip:      [10.5, 15.2, 20.1]           → avg = 15.27
zero:      [10.5, 0.0, 15.2, 0.0, 20.1] → avg = 9.16
lastValid: [10.5, 10.5, 15.2, 15.2, 20.1] → avg = 14.3
error:     Returns error if all values were NaN
```

`aggregationMethod` field allows to aggregate multiple metrics at the scaler side to get singular value required by KEDA.
> Prefer to use Yandex Monitoring’s built-in aggregation first and minimize aggregation on the scaler side.
- **`max`** (default): Use maximum value (useful for CPU hotspot detection)
- **`avg`**: Calculate average of all values
- **`sum`**: Sum all values (useful for RPS across zones)
- **`min`**: Use minimum value
- **`last`**: Use the most recent value

```
Processed values: [10.0, 15.0, 20.0, 25.0]

avg:  (10+15+20+25)/4 = 17.5
sum:  10+15+20+25 = 70.0
max:  25.0
min:  10.0
last: 25.0
```

`timeSeriesAggregation` is an intermediate aggregation step that happens before the main `aggregationMethod`.
If you have multiple metrics, each with its own time series data, `timeSeriesAggregation` aggregates each individual time series, then `aggregationMethod` aggregates across metrics.
> Prefer to use Yandex Monitoring’s built-in aggregation first and minimize aggregation on the scaler side.

```
Server 1: [80, 85, 90, 95, 100]
Server 2: [70, 75, 80, 85, 90]
Server 3: [60, 65, 70, 75, 80]

With timeSeriesAggregation="avg":
Server 1 → 90.0
Server 2 → 80.0
Server 3 → 70.0

Then aggregationMethod="max" → 90.0
```

### Logging Options

| Field | Description | Default | Options |
|-------|-------------|---------|---------|
| `logLevel` | Logging verbosity | `info` | `debug`, `info`, `warn`, `error`, `none` |

To troubleshoot scaling issues, enabling `logLevel: debug` can be helpful

Check the scaler pod logs with:
```bash
kubectl logs -l app.kubernetes.io/name=yc-keda-external-scaler -f
```

You'll see detailed information about:
- Downsampling configuration used
- Raw metric responses from Yandex Monitoring
- NaN value handling
- Aggregation steps and results
- Final metric values returned to KEDA
