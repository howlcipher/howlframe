from pydantic import BaseModel, Field
from typing import List, Dict, Union, Literal, Annotated, Any


class AchieveInstruction(BaseModel):
    op: Literal["ACHIEVE"]
    pass


class AppendInstruction(BaseModel):
    op: Literal["APPEND"]
    string_operand: str


class AttrEscapeInstruction(BaseModel):
    op: Literal["ATTR_ESCAPE"]
    pass


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


class EncodeJsonInstruction(BaseModel):
    op: Literal["ENCODE_JSON"]
    pass


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


class GrantedInstruction(BaseModel):
    op: Literal["GRANTED"]
    pass


class HtmlEscapeInstruction(BaseModel):
    op: Literal["HTML_ESCAPE"]
    pass


class HttpRequestInstruction(BaseModel):
    op: Literal["HTTP_REQUEST"]
    int_operand: int


class HttpReqBodyInstruction(BaseModel):
    op: Literal["HTTP_REQ_BODY"]
    pass


class HttpReqHeaderInstruction(BaseModel):
    op: Literal["HTTP_REQ_HEADER"]
    pass


class HttpReqIdInstruction(BaseModel):
    op: Literal["HTTP_REQ_ID"]
    pass


class HttpReqMethodInstruction(BaseModel):
    op: Literal["HTTP_REQ_METHOD"]
    pass


class HttpReqPathInstruction(BaseModel):
    op: Literal["HTTP_REQ_PATH"]
    pass


class HttpReqQueryInstruction(BaseModel):
    op: Literal["HTTP_REQ_QUERY"]
    pass


class HttpReqUrlPathInstruction(BaseModel):
    op: Literal["HTTP_REQ_URL_PATH"]
    pass


class HttpResHeaderInstruction(BaseModel):
    op: Literal["HTTP_RES_HEADER"]
    pass


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


class IsNilInstruction(BaseModel):
    op: Literal["IS_NIL"]
    pass


class JumpInstruction(BaseModel):
    op: Literal["JUMP"]
    int_operand: int


class JumpIfFalseInstruction(BaseModel):
    op: Literal["JUMP_IF_FALSE"]
    int_operand: int


class ListGetInstruction(BaseModel):
    op: Literal["LIST_GET"]
    string_operand: str


class ListLenInstruction(BaseModel):
    op: Literal["LIST_LEN"]
    pass


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


class MapKeysInstruction(BaseModel):
    op: Literal["MAP_KEYS"]
    pass


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


class RandHexInstruction(BaseModel):
    op: Literal["RAND_HEX"]
    pass


class ReadFileInstruction(BaseModel):
    op: Literal["READ_FILE"]
    pass


class ReadLineInstruction(BaseModel):
    op: Literal["READ_LINE"]
    pass


class ReadStdinInstruction(BaseModel):
    op: Literal["READ_STDIN"]
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


class SecureEqualInstruction(BaseModel):
    op: Literal["SECURE_EQUAL"]
    pass


class SetVarInstruction(BaseModel):
    op: Literal["SET_VAR"]
    string_operand: str


class Sha256HexInstruction(BaseModel):
    op: Literal["SHA256_HEX"]
    pass


class SleepInstruction(BaseModel):
    op: Literal["SLEEP"]
    pass


class SpawnInstruction(BaseModel):
    op: Literal["SPAWN"]
    int_operand: int


class SpawnAgentInstruction(BaseModel):
    op: Literal["SPAWN_AGENT"]
    string_operand: str
    int_operand: int


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


class StoreKeysInstruction(BaseModel):
    op: Literal["STORE_KEYS"]
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


class TimeNowInstruction(BaseModel):
    op: Literal["TIME_NOW"]
    pass


class TryLetInstruction(BaseModel):
    op: Literal["TRY_LET"]
    string_operand: str
    string_operand_2: str
    int_operand: int


class Utf8ValidInstruction(BaseModel):
    op: Literal["UTF8_VALID"]
    pass


class WriteFileInstruction(BaseModel):
    op: Literal["WRITE_FILE"]
    pass


Instruction = Annotated[
    Union[
        AchieveInstruction,
        AppendInstruction,
        AttrEscapeInstruction,
        BinopInstruction,
        CallInstruction,
        CliArgsInstruction,
        CliArgsGetInstruction,
        ConfidenceInstruction,
        ConvertInstruction,
        DbConnectInstruction,
        EncodeJsonInstruction,
        EnvInstruction,
        EphemeralCircuitInstruction,
        ExecInstruction,
        ExitInstruction,
        FetchInstruction,
        ForInitInstruction,
        ForNextInstruction,
        GrantedInstruction,
        HtmlEscapeInstruction,
        HttpRequestInstruction,
        HttpReqBodyInstruction,
        HttpReqHeaderInstruction,
        HttpReqIdInstruction,
        HttpReqMethodInstruction,
        HttpReqPathInstruction,
        HttpReqQueryInstruction,
        HttpReqUrlPathInstruction,
        HttpResHeaderInstruction,
        HttpRouteInstruction,
        HttpServerServeInstruction,
        HttpServerStartInstruction,
        IsNilInstruction,
        JumpInstruction,
        JumpIfFalseInstruction,
        ListGetInstruction,
        ListLenInstruction,
        LlmGenerateInstruction,
        LoadConstInstruction,
        LoadVarInstruction,
        MakeDictInstruction,
        MakeListInstruction,
        MapDeleteInstruction,
        MapGetInstruction,
        MapKeysInstruction,
        MapSetInstruction,
        MkdirInstruction,
        NeuralCircuitInstruction,
        ParseJsonInstruction,
        PrintInstruction,
        RandHexInstruction,
        ReadFileInstruction,
        ReadLineInstruction,
        ReadStdinInstruction,
        RegexMatchInstruction,
        ResInstruction,
        ResJsonInstruction,
        ReturnInstruction,
        SecureEqualInstruction,
        SetVarInstruction,
        Sha256HexInstruction,
        SleepInstruction,
        SpawnInstruction,
        SpawnAgentInstruction,
        SqlQueryInstruction,
        StderrInstruction,
        StoreDeleteInstruction,
        StoreGetInstruction,
        StoreKeysInstruction,
        StoreOpenInstruction,
        StorePutInstruction,
        StoreVarInstruction,
        StrJoinInstruction,
        StrSplitInstruction,
        TaskInstruction,
        TimeNowInstruction,
        TryLetInstruction,
        Utf8ValidInstruction,
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
