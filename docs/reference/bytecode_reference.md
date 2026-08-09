# HowlFrame Bytecode Instruction Reference

| Opcode | Operands | Pops | Pushes | Capability | Description |
|---|---|---|---|---|---|
| `ACHIEVE` |  | 2 | 1 | network | Achieves a target state given a constraint |
| `APPEND` | string | 1 | 0 |  | Appends to a list |
| `BINOP` | string | 2 | 1 |  | Binary operation |
| `CALL` | string, int64 | var | 1 |  | Calls a function |
| `CLI_ARGS` |  | 0 | 1 |  | Gets command line arguments |
| `CLI_ARGS_GET` |  | 1 | 1 |  | Gets a specific command line argument |
| `CONFIDENCE` |  | 1 | 1 |  | Returns confidence score for LLM generate |
| `CONVERT` | string | 1 | 1 |  | Type conversion |
| `DB_CONNECT` | string, string, string | 0 | 0 | database | Connects to a database |
| `ENV` |  | 1 | 1 | environment | Gets an environment variable |
| `EPHEMERAL_CIRCUIT` | int64 | var | 1 |  | Generates an ephemeral specialized model, executes it, and discards it |
| `EXEC` | int64 | var | 1 | process | Executes a shell command |
| `EXIT` |  | 1 | 0 |  | Exits the process with a given status code |
| `FETCH` |  | 2 | 1 | network | Fetches a URL |
| `FOR_INIT` |  | 1 | 1 |  | Initializes a for loop |
| `FOR_NEXT` | string, int64 | 0 | 0 |  | Next iteration of a for loop |
| `HTTP_ROUTE` | string, string, int64 | 0 | 0 | network | Registers an HTTP route |
| `HTTP_SERVER_SERVE` |  | 0 | 0 | network | Serves HTTP requests |
| `HTTP_SERVER_START` | string | 0 | 0 | network | Starts an HTTP server |
| `JUMP` | int64 | 0 | 0 |  | Unconditional jump |
| `JUMP_IF_FALSE` | int64 | 1 | 0 |  | Jumps if top of stack is false |
| `LIST_GET` | string | 1 | 1 |  | Gets a value from a list |
| `LLM_GENERATE` | string | 1 | 1 | network | Generates text using an LLM |
| `LOAD_CONST` | value | 0 | 1 |  | Loads a constant value onto the stack |
| `LOAD_VAR` | string | 0 | 1 |  | Loads a variable onto the stack |
| `MAKE_DICT` | int64 | var | 1 |  | Creates a dictionary |
| `MAKE_LIST` | int64 | var | 1 |  | Creates a list |
| `MAP_DELETE` | string | 1 | 0 |  | Deletes a key from a dictionary |
| `MAP_GET` | string | 1 | 1 |  | Gets a value from a dictionary |
| `MAP_SET` | string | 2 | 0 |  | Sets a key in a dictionary |
| `MKDIR` |  | 1 | 0 | filesystem | Creates a directory |
| `NEURAL_CIRCUIT` | int64 | var | 1 |  | Executes an LLM logic circuit with a given number of inputs and an instruction |
| `PARSE_JSON` | string | 0 | 1 |  | Parses JSON from a string variable |
| `PRINT` | int64 | var | 0 |  | Prints values to standard output |
| `READ_FILE` |  | 1 | 1 | filesystem | Reads a file |
| `READ_LINE` |  | 0 | 1 |  | Reads a line from standard input |
| `REGEX_MATCH` |  | 2 | 1 |  | Matches a string against a regex |
| `RES` |  | 2 | 0 | network | Sends an HTTP response |
| `RES_JSON` |  | 2 | 0 | network | Sends a JSON HTTP response |
| `RETURN` |  | 1 | 0 |  | Returns from a function |
| `SET_VAR` | string | 1 | 0 |  | Pops a value and updates an existing variable |
| `SLEEP` |  | 1 | 0 |  | Sleeps for a duration |
| `SPAWN` | int64 | 0 | 0 | process | Spawns a background task |
| `SPAWN_AGENT` | string | 1 | 0 | process | Spawns an autonomous subagent |
| `SQL_QUERY` | string, string | 0 | 1 | database | Executes a SQL query |
| `STDERR` |  | 1 | 0 |  | Prints a value to standard error |
| `STORE_DELETE` | string | 1 | 0 | database | Deletes a structured record by key |
| `STORE_GET` | string | 1 | 1 | database | Fetches a structured record by key |
| `STORE_OPEN` | string, string | 0 | 0 | database | Creates or attaches a named in-memory store handle |
| `STORE_PUT` | string | 2 | 0 | database | Upserts a structured record by key |
| `STORE_VAR` | string | 1 | 0 |  | Pops a value and stores it in a new variable |
| `STR_JOIN` |  | 2 | 1 |  | Joins a list of strings |
| `STR_SPLIT` |  | 2 | 1 |  | Splits a string |
| `TASK` | string | 0 | 1 |  | Defines a task for a subagent |
| `TRY_LET` | string, string, int64 | 0 | 0 |  | Try block with let binding |
| `WRITE_FILE` |  | 2 | 0 | filesystem | Writes to a file |
