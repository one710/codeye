package acp

const ProtocolVersion = 1

// Agent methods from ACP stable meta.json.
const (
	MethodInitialize          = "initialize"
	MethodAuthenticate        = "authenticate"
	MethodSessionCancel       = "session/cancel"
	MethodSessionList         = "session/list"
	MethodSessionLoad         = "session/load"
	MethodSessionNew          = "session/new"
	MethodSessionPrompt       = "session/prompt"
	MethodSessionSetConfig    = "session/set_config_option"
	MethodSessionSetMode      = "session/set_mode"
	MethodSessionRequestPerm  = "session/request_permission"
	MethodSessionUpdate       = "session/update"
	MethodFSReadTextFile      = "fs/read_text_file"
	MethodFSWriteTextFile     = "fs/write_text_file"
	MethodTerminalCreate      = "terminal/create"
	MethodTerminalKill        = "terminal/kill"
	MethodTerminalOutput      = "terminal/output"
	MethodTerminalRelease     = "terminal/release"
	MethodTerminalWaitForExit = "terminal/wait_for_exit"
)
