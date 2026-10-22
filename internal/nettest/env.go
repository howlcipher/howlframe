package nettest

// EnvRefused are process:env= names every target must refuse (#166, #172,
// #177). The list is independent of capability.CodeLoadingEnvNames on
// purpose: deleting a table row must fail the parity tests.
var EnvRefused = []string{
	"LD_PRELOAD", "ld_library_path", "LD_AUDIT", "DYLD_INSERT_LIBRARIES",
	"PYTHONPATH", "PYTHONHOME", "PYTHONSTARTUP", "pythonhome", "PYTHONWARNINGS", "PYTHONBREAKPOINT",
	"NODE_OPTIONS", "NODE_PATH", "NODE_EXTRA_CA_CERTS", "node_options",
	"BASH_ENV", "ENV", "SHELLOPTS", "BASHOPTS", "PS4", "PROMPT_COMMAND", "IFS", "BASH_FUNC_x",
	"PERL5LIB", "PERLLIB", "PERL5OPT", "PERL5DB", "RUBYOPT", "RUBYLIB",
	"GOFLAGS", "GODEBUG", "GOROOT", "JAVA_TOOL_OPTIONS", "_JAVA_OPTIONS", "JDK_JAVA_OPTIONS", "CLASSPATH",
	"GCONV_PATH", "LOCPATH", "HOSTALIASES",
	"GIT_DIR", "GIT_EXEC_PATH", "GIT_SSH_COMMAND", "GIT_CONFIG_GLOBAL", "git_askpass",
	"HOME", "home", "XDG_CONFIG_HOME", "XDG_CONFIG_DIRS", "PATH", "path",
	"SSL_CERT_FILE", "SSL_CERT_DIR", "LUA_INIT", "LUA_PATH",
	// #182 and the 2026-10-21 audit
	"OPENSSL_CONF", "OPENSSL_MODULES", "GLIBC_TUNABLES", "MALLOC_CHECK_",
	"HTTP_PROXY", "https_proxy", "ALL_PROXY", "no_proxy", "NODE_USE_ENV_PROXY",
	"GEM_PATH", "GEM_HOME", "RUBYGEMS_GEMDEPS", "BUNDLE_GEMFILE", "PHPRC", "ERL_LIBS",
	"TMPDIR", "LESSOPEN", "PAGER", "EDITOR", "VISUAL", "SHELL", "SSH_ASKPASS", "RES_OPTIONS",
	"PIP_INDEX_URL", "NPM_CONFIG_REGISTRY", "CARGO_HOME", "GOPROXY", "GOMODCACHE", "pythonioencoding",
}

// EnvAllowed must still pass through on every target.
var EnvAllowed = []string{
	"LANG", "TZ", "HOWL_TOKEN", "LOGNAME", "PATHNAME", "HOMEDIR", "GITHUB_SHA", "XDG_RUNTIME_DIR",
	// inert data and application names that sit next to the #182 families
	"LC_ALL", "TERM", "NO_COLOR", "CI", "SOURCE_DATE_EPOCH", "USER", "HF_TEST_PASS",
	"GOAL_ID", "MYPROXY", "APP_PROXY_URL", "TEMPLATE_DIR", "EDITOR_THEME",
}
