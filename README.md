# NRF

This repository is the team NRF fork used by the distributed NWDAF work.

## Release 18 NWDAF Discovery Boundary

The pinned free5GC generated OpenAPI module is Release 17, so Release 18
NWDAF/ADRF profile fields are represented by repository-local compatibility
types under `internal/compat/nrf`. Registration validates and persists the
complete JSON profile without dropping:

- `nwdafInfo` and `nwdafInfoList`;
- `mlAnalyticsList`, including `flCapabilityType`,
  `mlModelInterInfo`, TAI, S-NSSAI, NF type, and NF set filters;
- `adrfInfoList` model/data storage indicators.

GET, registration responses, profile-change notifications, and compatible
discovery responses retain those fields. Existing Release 17 fields continue
to use generated free5GC models.

NF Discovery supports the Release 18 filters needed by the current NWDAF
design:

- `nwdaf-event-list`;
- `ml-analytics-info-list`;
- `internal-group-identity`;
- `ml-model-storage-ind`;
- `data-storage-ind`.

Attributes in one `ml-analytics-info-list` entry are matched against one
profile entry with AND semantics. Multiple query entries use OR semantics;
list-valued attributes match by overlap. `FL_SERVER_AND_CLIENT` satisfies a
server-only or client-only query. Malformed or target-incompatible filters
return `400`. Compatible responses advertise `nrfSupportedFeatures:
"800010000"` for the implemented Query-eNA-PH2/PH3 behavior.

This fork does not add NWDAF Model Training routes, client ranking, or
priority/capacity selection.

## Verification

```bash
go test ./...
go test -race ./internal/sbi/consumer ./internal/sbi/processor
go build ./...
```
