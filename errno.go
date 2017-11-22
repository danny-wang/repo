package main

type ErrCode int

const (
	ERR_OK                   ErrCode = 0
	ERR_FILE_EXIST_DIR       ErrCode = 1
	ERR_HTTP_GET_CONTENT     ErrCode = 10
	ERR_REQ_PARAMETER_EXPIRE ErrCode = 20
	ERR_REQ_PARAMETER_PATH   ErrCode = 21
	ERR_UPDATE_DB            ErrCode = 30
	ERR_READ_DB              ErrCode = 31
	ERR_MKDIR                ErrCode = 40
	ERR_OPEN_FILE            ErrCode = 50
	ERR_FILE_NOT_IN_DB       ErrCode = 60
	ERR_FILE_NOT_EXIST       ErrCode = 70
)

type ErrInfo struct {
	Status ErrCode
	Msg    string
}

func (e ErrCode) String() string {
	switch e {
	case ERR_OK:
		return "OK"
	case ERR_FILE_EXIST_DIR:
		return "File exist, it is directory"
	case ERR_HTTP_GET_CONTENT:
		return "get HTTP content error"
	case ERR_REQ_PARAMETER_EXPIRE:
		return "request expired time format error"
	case ERR_REQ_PARAMETER_PATH:
		return "request path error"
	case ERR_UPDATE_DB:
		return "update db error"
	case ERR_READ_DB:
		return "read db error"
	case ERR_MKDIR:
		return "mkdir error"
	case ERR_OPEN_FILE:
		return "open file error"
	case ERR_FILE_NOT_IN_DB:
		return "file not exist in db"
	case ERR_FILE_NOT_EXIST:
		return "file not exist"
	default:
		return "unknown error"
	}
}

func MakeErrInfo(s ErrCode) ErrInfo {
	return ErrInfo{Status: s, Msg: s.String()}
}
