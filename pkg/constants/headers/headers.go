package headers

// These constants define the keys for headers used in API requests.
const (
	APIKeyHeader    = "X-API-Key"    // The API key used for authentication
	SecretKeyHeader = "X-Secret-Key" // The secret key used for authentication
	PeerHeader      = "X-Peer"       // Information about the requesting peer
	AccountIDHeader = "X-Account-ID" // The account id of the requesting peer
	// ActAsAPIKeyHeader lets an already-authenticated peer (basic auth + X-Peer)
	// run a request on behalf of one of the account's API credentials without
	// holding that credential's secret. The credential is resolved by api key
	// within X-Account-ID and is subject to the same archived/expiry/scope checks
	// as a direct X-API-Key/X-Secret-Key call. Used by the dashboard API playground.
	ActAsAPIKeyHeader = "X-Act-As-API-Key"
)

// These constants define the values for the PeerHeader key.
const (
	PeerHeaderValue    = "peer" // Indicates that the request is coming from a peer node
	PeerHeaderCMDValue = "cmd"  // Indicates that the request is a command sent to a peer node
)

// LocationHeader is the standard HTTP response header used for redirects.
const LocationHeader = "Location"
