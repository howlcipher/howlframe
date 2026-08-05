package bytecode

import (
	"zero/internal/capability"
)

type Opcode uint16

const (
	OpUnknown Opcode = iota
	OpLoadConst
	OpLoadVar
	OpStoreVar
	OpSetVar
	OpTryLet
	OpDbConnect
	OpSqlQuery
	OpFetch
	OpReadFile
	OpWriteFile
	OpMkdir
	OpExec
	OpParseJson
	OpSpawn
	OpRes
	OpResJson
	OpHttpServerStart
	OpHttpRoute
	OpHttpServerServe
	OpConfidence
	OpLlmGenerate
	OpJumpIfFalse
	OpJump
	OpForInit
	OpForNext
	OpReturn
	OpPrint
	OpCall
	OpBinop
	OpConvert
	OpStrSplit
	OpStrJoin
	OpRegexMatch
	OpMakeList
	OpMakeDict
	OpAppend
	OpMapSet
	OpMapDelete
	OpMapGet
	OpListGet
	OpCliArgs
	OpCliArgsGet
	OpSleep
	OpEnv
	OpSpawnAgent
	OpTask
	OpAchieve
	OpNeuralCircuit
	OpEphemeralCircuit
	OpStoreOpen
	OpStorePut
	OpStoreGet
	OpStoreDelete
)

type OperandType string

const (
	OperandString OperandType = "string"
	OperandInt    OperandType = "int64"
	OperandValue  OperandType = "value" // can be string, float64, bool, nil
)

type OpcodeSpec struct {
	Code        Opcode
	Name        string
	Operands    []OperandType
	Pops        int // -1 means variable (based on operand)
	Pushes      int
	Capability  capability.Capability
	Description string
}

var Registry = map[Opcode]OpcodeSpec{
	OpLoadConst:        {Code: OpLoadConst, Name: "LOAD_CONST", Operands: []OperandType{OperandValue}, Pops: 0, Pushes: 1, Description: "Loads a constant value onto the stack"},
	OpLoadVar:          {Code: OpLoadVar, Name: "LOAD_VAR", Operands: []OperandType{OperandString}, Pops: 0, Pushes: 1, Description: "Loads a variable onto the stack"},
	OpStoreVar:         {Code: OpStoreVar, Name: "STORE_VAR", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 0, Description: "Pops a value and stores it in a new variable"},
	OpSetVar:           {Code: OpSetVar, Name: "SET_VAR", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 0, Description: "Pops a value and updates an existing variable"},
	OpTryLet:           {Code: OpTryLet, Name: "TRY_LET", Operands: []OperandType{OperandString, OperandString, OperandInt}, Pops: 0, Pushes: 0, Description: "Try block with let binding"},
	OpDbConnect:        {Code: OpDbConnect, Name: "DB_CONNECT", Operands: []OperandType{OperandString, OperandString, OperandString}, Pops: 0, Pushes: 0, Capability: capability.Database, Description: "Connects to a database"},
	OpSqlQuery:         {Code: OpSqlQuery, Name: "SQL_QUERY", Operands: []OperandType{OperandString, OperandString}, Pops: 0, Pushes: 1, Capability: capability.Database, Description: "Executes a SQL query"},
	OpFetch:            {Code: OpFetch, Name: "FETCH", Operands: []OperandType{}, Pops: 2, Pushes: 1, Capability: capability.Network, Description: "Fetches a URL"},
	OpReadFile:         {Code: OpReadFile, Name: "READ_FILE", Operands: []OperandType{}, Pops: 1, Pushes: 1, Capability: capability.Filesystem, Description: "Reads a file"},
	OpWriteFile:        {Code: OpWriteFile, Name: "WRITE_FILE", Operands: []OperandType{}, Pops: 2, Pushes: 0, Capability: capability.Filesystem, Description: "Writes to a file"},
	OpMkdir:            {Code: OpMkdir, Name: "MKDIR", Operands: []OperandType{}, Pops: 1, Pushes: 0, Capability: capability.Filesystem, Description: "Creates a directory"},
	OpExec:             {Code: OpExec, Name: "EXEC", Operands: []OperandType{OperandInt}, Pops: -1, Pushes: 1, Capability: capability.Process, Description: "Executes a shell command"},
	OpParseJson:        {Code: OpParseJson, Name: "PARSE_JSON", Operands: []OperandType{OperandString}, Pops: 0, Pushes: 1, Description: "Parses JSON from a string variable"},
	OpSpawn:            {Code: OpSpawn, Name: "SPAWN", Operands: []OperandType{OperandInt}, Pops: 0, Pushes: 0, Capability: capability.Process, Description: "Spawns a background task"},
	OpRes:              {Code: OpRes, Name: "RES", Operands: []OperandType{}, Pops: 2, Pushes: 0, Capability: capability.Network, Description: "Sends an HTTP response"},
	OpResJson:          {Code: OpResJson, Name: "RES_JSON", Operands: []OperandType{}, Pops: 2, Pushes: 0, Capability: capability.Network, Description: "Sends a JSON HTTP response"},
	OpHttpServerStart:  {Code: OpHttpServerStart, Name: "HTTP_SERVER_START", Operands: []OperandType{OperandString}, Pops: 0, Pushes: 0, Capability: capability.Network, Description: "Starts an HTTP server"},
	OpHttpRoute:        {Code: OpHttpRoute, Name: "HTTP_ROUTE", Operands: []OperandType{OperandString, OperandString, OperandInt}, Pops: 0, Pushes: 0, Capability: capability.Network, Description: "Registers an HTTP route"},
	OpHttpServerServe:  {Code: OpHttpServerServe, Name: "HTTP_SERVER_SERVE", Operands: []OperandType{}, Pops: 0, Pushes: 0, Capability: capability.Network, Description: "Serves HTTP requests"},
	OpConfidence:       {Code: OpConfidence, Name: "CONFIDENCE", Operands: []OperandType{}, Pops: 1, Pushes: 1, Description: "Returns confidence score for LLM generate"},
	OpLlmGenerate:      {Code: OpLlmGenerate, Name: "LLM_GENERATE", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 1, Capability: capability.Network, Description: "Generates text using an LLM"},
	OpJumpIfFalse:      {Code: OpJumpIfFalse, Name: "JUMP_IF_FALSE", Operands: []OperandType{OperandInt}, Pops: 1, Pushes: 0, Description: "Jumps if top of stack is false"},
	OpJump:             {Code: OpJump, Name: "JUMP", Operands: []OperandType{OperandInt}, Pops: 0, Pushes: 0, Description: "Unconditional jump"},
	OpForInit:          {Code: OpForInit, Name: "FOR_INIT", Operands: []OperandType{}, Pops: 1, Pushes: 1, Description: "Initializes a for loop"},
	OpForNext:          {Code: OpForNext, Name: "FOR_NEXT", Operands: []OperandType{OperandString, OperandInt}, Pops: 0, Pushes: 0, Description: "Next iteration of a for loop"},
	OpReturn:           {Code: OpReturn, Name: "RETURN", Operands: []OperandType{}, Pops: 1, Pushes: 0, Description: "Returns from a function"},
	OpPrint:            {Code: OpPrint, Name: "PRINT", Operands: []OperandType{OperandInt}, Pops: -1, Pushes: 0, Description: "Prints values to standard output"},
	OpCall:             {Code: OpCall, Name: "CALL", Operands: []OperandType{OperandString, OperandInt}, Pops: -1, Pushes: 1, Description: "Calls a function"},
	OpBinop:            {Code: OpBinop, Name: "BINOP", Operands: []OperandType{OperandString}, Pops: 2, Pushes: 1, Description: "Binary operation"},
	OpConvert:          {Code: OpConvert, Name: "CONVERT", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 1, Description: "Type conversion"},
	OpStrSplit:         {Code: OpStrSplit, Name: "STR_SPLIT", Operands: []OperandType{}, Pops: 2, Pushes: 1, Description: "Splits a string"},
	OpStrJoin:          {Code: OpStrJoin, Name: "STR_JOIN", Operands: []OperandType{}, Pops: 2, Pushes: 1, Description: "Joins a list of strings"},
	OpRegexMatch:       {Code: OpRegexMatch, Name: "REGEX_MATCH", Operands: []OperandType{}, Pops: 2, Pushes: 1, Description: "Matches a string against a regex"},
	OpMakeList:         {Code: OpMakeList, Name: "MAKE_LIST", Operands: []OperandType{OperandInt}, Pops: -1, Pushes: 1, Description: "Creates a list"},
	OpMakeDict:         {Code: OpMakeDict, Name: "MAKE_DICT", Operands: []OperandType{OperandInt}, Pops: -1, Pushes: 1, Description: "Creates a dictionary"},
	OpAppend:           {Code: OpAppend, Name: "APPEND", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 0, Description: "Appends to a list"},
	OpMapSet:           {Code: OpMapSet, Name: "MAP_SET", Operands: []OperandType{OperandString}, Pops: 2, Pushes: 0, Description: "Sets a key in a dictionary"},
	OpMapDelete:        {Code: OpMapDelete, Name: "MAP_DELETE", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 0, Description: "Deletes a key from a dictionary"},
	OpMapGet:           {Code: OpMapGet, Name: "MAP_GET", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 1, Description: "Gets a value from a dictionary"},
	OpListGet:          {Code: OpListGet, Name: "LIST_GET", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 1, Description: "Gets a value from a list"},
	OpCliArgs:          {Code: OpCliArgs, Name: "CLI_ARGS", Operands: []OperandType{}, Pops: 0, Pushes: 1, Description: "Gets command line arguments"},
	OpCliArgsGet:       {Code: OpCliArgsGet, Name: "CLI_ARGS_GET", Operands: []OperandType{}, Pops: 1, Pushes: 1, Description: "Gets a specific command line argument"},
	OpSleep:            {Code: OpSleep, Name: "SLEEP", Operands: []OperandType{}, Pops: 1, Pushes: 0, Description: "Sleeps for a duration"},
	OpEnv:              {Code: OpEnv, Name: "ENV", Operands: []OperandType{}, Pops: 1, Pushes: 1, Capability: capability.Environment, Description: "Gets an environment variable"},
	OpSpawnAgent:       {Code: OpSpawnAgent, Name: "SPAWN_AGENT", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 0, Capability: capability.Process, Description: "Spawns an autonomous subagent"},
	OpTask:             {Code: OpTask, Name: "TASK", Operands: []OperandType{OperandString}, Pops: 0, Pushes: 1, Description: "Defines a task for a subagent"},
	OpAchieve:          {Code: OpAchieve, Name: "ACHIEVE", Operands: []OperandType{}, Pops: 2, Pushes: 1, Description: "Achieves a target state given a constraint"},
	OpNeuralCircuit:    {Code: OpNeuralCircuit, Name: "NEURAL_CIRCUIT", Operands: []OperandType{OperandInt}, Pops: -1, Pushes: 1, Description: "Executes an LLM logic circuit with a given number of inputs and an instruction"},
	OpEphemeralCircuit: {Code: OpEphemeralCircuit, Name: "EPHEMERAL_CIRCUIT", Operands: []OperandType{OperandInt}, Pops: -1, Pushes: 1, Description: "Generates an ephemeral specialized model, executes it, and discards it"},
	OpStoreOpen:        {Code: OpStoreOpen, Name: "STORE_OPEN", Operands: []OperandType{OperandString, OperandString}, Pops: 0, Pushes: 0, Capability: capability.Database, Description: "Creates or attaches a named in-memory store handle"},
	OpStorePut:         {Code: OpStorePut, Name: "STORE_PUT", Operands: []OperandType{OperandString}, Pops: 2, Pushes: 0, Capability: capability.Database, Description: "Upserts a structured record by key"},
	OpStoreGet:         {Code: OpStoreGet, Name: "STORE_GET", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 1, Capability: capability.Database, Description: "Fetches a structured record by key"},
	OpStoreDelete:      {Code: OpStoreDelete, Name: "STORE_DELETE", Operands: []OperandType{OperandString}, Pops: 1, Pushes: 0, Capability: capability.Database, Description: "Deletes a structured record by key"},
}

func NameToOpcode(name string) (Opcode, bool) {
	for code, spec := range Registry {
		if spec.Name == name {
			return code, true
		}
	}
	return OpUnknown, false
}
