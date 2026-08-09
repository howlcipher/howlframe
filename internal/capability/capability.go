package capability

// Capability represents an explicitly granted permission to perform a class of effects.
type Capability string

const (
	None        Capability = ""
	Network     Capability = "network"
	Filesystem  Capability = "filesystem"
	Process     Capability = "process"
	Environment Capability = "environment"
	Database    Capability = "database"
)

// All returns a deterministic list of all known valid capabilities.
func All() []Capability {
	return []Capability{
		Network,
		Filesystem,
		Process,
		Environment,
		Database,
	}
}

// ForConstruct returns the capability required by a canonical HowlFrame operation or construct name.
// If the construct does not require a capability, it returns None.
func ForConstruct(name string) Capability {
	switch name {
	case "db_connect", "sql_query", "store_open", "store_put", "store_get", "store_delete":
		return Database
	case "fetch", "res", "res_json", "http_server_start", "http_route", "http_server_serve", "llm_generate", "achieve":
		return Network
	case "read_file", "write_file", "mkdir":
		return Filesystem
	case "exec", "spawn", "spawn_agent":
		return Process
	case "env":
		return Environment
	}
	return None
}
