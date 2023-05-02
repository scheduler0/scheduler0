package headers

// These constants define the keys for headers used in API requests.
const (
	APIKeyHeader      = "x-api-key"    // The API key used for authentication
	SecretKeyHeader   = "x-secret-key" // The secret key used for authentication
	PeerHeader        = "x-peer"       // Information about the requesting peer
	PeerAddressHeader = "peer-address" // The address of the requesting peer
)

// These constants define the values for the PeerHeader key.
const (
	PeerHeaderValue    = "peer" // Indicates that the request is coming from a peer node
	PeerHeaderCMDValue = "cmd"  // Indicates that the request is a command sent to a peer node
)
