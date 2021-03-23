package metadata

const (
	// Indicates the subscription ID the resource belongs to
	LabelKeySubscription = "ddev.live/subscription"
	// Indicates the customer the resource belongs to
	LabelKeyCustomer = "ddev.live/customer"
	// Indiciates the workspace the resource belongs to
	LabelKeyWorkspace = "ddev.live/workspace"

	ClaimKeyDefaultWorkspace = "default_workspace"

	// Indicates the firebase token for the request
	HeaderAuthToken = "x-auth-token"
	// Indicates the workspace scope for the request
	HeaderWorkspace = "x-ddev-workspace"
)
