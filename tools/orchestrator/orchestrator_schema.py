from pydantic import BaseModel, Field
from typing import List, Dict, Union, Literal, Annotated, Any

class AchieveInstruction(BaseModel):
    op: Literal["ACHIEVE"]
    pass

class AppendInstruction(BaseModel):
    op: Literal["APPEND"]
    string_operand: str

class BinopInstruction(BaseModel):
    op: Literal["BINOP"]
    string_operand: str

class CallInstruction(BaseModel):
    op: Literal["CALL"]
    string_operand: str
    int_operand: int

class CliArgsInstruction(BaseModel):
    op: Literal["CLI_ARGS"]
    pass

class CliArgsGetInstruction(BaseModel):
    op: Literal["CLI_ARGS_GET"]
    pass

class ConfidenceInstruction(BaseModel):
    op: Literal["CONFIDENCE"]
    pass

class ConvertInstruction(BaseModel):
    op: Literal["CONVERT"]
    string_operand: str

class DbConnectInstruction(BaseModel):
    op: Literal["DB_CONNECT"]
    string_operand: str
    string_operand_2: str
    string_operand_3: str

class EnvInstruction(BaseModel):
    op: Literal["ENV"]
    pass

class EphemeralCircuitInstruction(BaseModel):
    op: Literal["EPHEMERAL_CIRCUIT"]
    int_operand: int

class ExecInstruction(BaseModel):
    op: Literal["EXEC"]
    int_operand: int

class ExitInstruction(BaseModel):
    op: Literal["EXIT"]
    pass

class FetchInstruction(BaseModel):
    op: Literal["FETCH"]
    pass

class ForInitInstruction(BaseModel):
    op: Literal["FOR_INIT"]
    pass

class ForNextInstruction(BaseModel):
    op: Literal["FOR_NEXT"]
    string_operand: str
    int_operand: int

class HttpRouteInstruction(BaseModel):
    op: Literal["HTTP_ROUTE"]
    string_operand: str
    string_operand_2: str
    int_operand: int

class HttpServerServeInstruction(BaseModel):
    op: Literal["HTTP_SERVER_SERVE"]
    pass

class HttpServerStartInstruction(BaseModel):
    op: Literal["HTTP_SERVER_START"]
    string_operand: str

class JumpInstruction(BaseModel):
    op: Literal["JUMP"]
    int_operand: int

class JumpIfFalseInstruction(BaseModel):
    op: Literal["JUMP_IF_FALSE"]
    int_operand: int

class ListGetInstruction(BaseModel):
    op: Literal["LIST_GET"]
    string_operand: str

class LlmGenerateInstruction(BaseModel):
    op: Literal["LLM_GENERATE"]
    string_operand: str

class LoadConstInstruction(BaseModel):
    op: Literal["LOAD_CONST"]
    value_operand: Any

class LoadVarInstruction(BaseModel):
    op: Literal["LOAD_VAR"]
    string_operand: str

class MakeDictInstruction(BaseModel):
    op: Literal["MAKE_DICT"]
    int_operand: int

class MakeListInstruction(BaseModel):
    op: Literal["MAKE_LIST"]
    int_operand: int

class MapDeleteInstruction(BaseModel):
    op: Literal["MAP_DELETE"]
    string_operand: str

class MapGetInstruction(BaseModel):
    op: Literal["MAP_GET"]
    string_operand: str

class MapSetInstruction(BaseModel):
    op: Literal["MAP_SET"]
    string_operand: str

class MkdirInstruction(BaseModel):
    op: Literal["MKDIR"]
    pass

class NeuralCircuitInstruction(BaseModel):
    op: Literal["NEURAL_CIRCUIT"]
    int_operand: int

class ParseJsonInstruction(BaseModel):
    op: Literal["PARSE_JSON"]
    string_operand: str

class PrintInstruction(BaseModel):
    op: Literal["PRINT"]
    int_operand: int

class ReadFileInstruction(BaseModel):
    op: Literal["READ_FILE"]
    pass

class ReadLineInstruction(BaseModel):
    op: Literal["READ_LINE"]
    pass

class RegexMatchInstruction(BaseModel):
    op: Literal["REGEX_MATCH"]
    pass

class ResInstruction(BaseModel):
    op: Literal["RES"]
    pass

class ResJsonInstruction(BaseModel):
    op: Literal["RES_JSON"]
    pass

class ReturnInstruction(BaseModel):
    op: Literal["RETURN"]
    pass

class SetVarInstruction(BaseModel):
    op: Literal["SET_VAR"]
    string_operand: str

class SleepInstruction(BaseModel):
    op: Literal["SLEEP"]
    pass

class SpawnInstruction(BaseModel):
    op: Literal["SPAWN"]
    int_operand: int

class SpawnAgentInstruction(BaseModel):
    op: Literal["SPAWN_AGENT"]
    string_operand: str

class SqlQueryInstruction(BaseModel):
    op: Literal["SQL_QUERY"]
    string_operand: str
    string_operand_2: str

class StderrInstruction(BaseModel):
    op: Literal["STDERR"]
    pass

class StoreDeleteInstruction(BaseModel):
    op: Literal["STORE_DELETE"]
    string_operand: str

class StoreGetInstruction(BaseModel):
    op: Literal["STORE_GET"]
    string_operand: str

class StoreOpenInstruction(BaseModel):
    op: Literal["STORE_OPEN"]
    string_operand: str
    string_operand_2: str

class StorePutInstruction(BaseModel):
    op: Literal["STORE_PUT"]
    string_operand: str

class StoreVarInstruction(BaseModel):
    op: Literal["STORE_VAR"]
    string_operand: str

class StrJoinInstruction(BaseModel):
    op: Literal["STR_JOIN"]
    pass

class StrSplitInstruction(BaseModel):
    op: Literal["STR_SPLIT"]
    pass

class TaskInstruction(BaseModel):
    op: Literal["TASK"]
    string_operand: str

class TryLetInstruction(BaseModel):
    op: Literal["TRY_LET"]
    string_operand: str
    string_operand_2: str
    int_operand: int

class WriteFileInstruction(BaseModel):
    op: Literal["WRITE_FILE"]
    pass

Instruction = Annotated[
    Union[
        AchieveInstruction,
        AppendInstruction,
        BinopInstruction,
        CallInstruction,
        CliArgsInstruction,
        CliArgsGetInstruction,
        ConfidenceInstruction,
        ConvertInstruction,
        DbConnectInstruction,
        EnvInstruction,
        EphemeralCircuitInstruction,
        ExecInstruction,
        ExitInstruction,
        FetchInstruction,
        ForInitInstruction,
        ForNextInstruction,
        HttpRouteInstruction,
        HttpServerServeInstruction,
        HttpServerStartInstruction,
        JumpInstruction,
        JumpIfFalseInstruction,
        ListGetInstruction,
        LlmGenerateInstruction,
        LoadConstInstruction,
        LoadVarInstruction,
        MakeDictInstruction,
        MakeListInstruction,
        MapDeleteInstruction,
        MapGetInstruction,
        MapSetInstruction,
        MkdirInstruction,
        NeuralCircuitInstruction,
        ParseJsonInstruction,
        PrintInstruction,
        ReadFileInstruction,
        ReadLineInstruction,
        RegexMatchInstruction,
        ResInstruction,
        ResJsonInstruction,
        ReturnInstruction,
        SetVarInstruction,
        SleepInstruction,
        SpawnInstruction,
        SpawnAgentInstruction,
        SqlQueryInstruction,
        StderrInstruction,
        StoreDeleteInstruction,
        StoreGetInstruction,
        StoreOpenInstruction,
        StorePutInstruction,
        StoreVarInstruction,
        StrJoinInstruction,
        StrSplitInstruction,
        TaskInstruction,
        TryLetInstruction,
        WriteFileInstruction,
    ],
    Field(discriminator="op")
]

class Function(BaseModel):
    params: List[str]
    instructions: List[Instruction]

class BytecodeProgram(BaseModel):
    version: int
    functions: Dict[str, Function]
    main: List[Instruction]
