package database

import "fmt"

type ErrorCode int
const (
	ErrDbTypeRegistered ErrorCode = iota
	ErrDbUnknownType
	ErrDbDoesNotExist
	ErrDbExists
	ErrDbNotOpen
	ErrDbAlreadyOpen
	ErrInvalid
	ErrCorruption
	ErrTxClosed
	ErrTxNotWritable
	ErrBucketNotFound
	ErrBucketExists
	ErrBucketNameRequired
	ErrKeyRequired
	ErrKeyTooLarge
	ErrValueTooLarge
	ErrIncompatibleValue
	ErrBlockNotFound
	ErrBlockExists
	ErrBlockRegionInvalid
	ErrDriverSpecific
	numErrorCodes
)

var errorCodeStrings = map[ErrorCode]string{
	ErrDbTypeRegistered:   "ErrDbTypeRegistered",
	ErrDbUnknownType:      "ErrDbUnknownType",
	ErrDbDoesNotExist:     "ErrDbDoesNotExist",
	ErrDbExists:           "ErrDbExists",
	ErrDbNotOpen:          "ErrDbNotOpen",
	ErrDbAlreadyOpen:      "ErrDbAlreadyOpen",
	ErrInvalid:            "ErrInvalid",
	ErrCorruption:         "ErrCorruption",
	ErrTxClosed:           "ErrTxClosed",
	ErrTxNotWritable:      "ErrTxNotWritable",
	ErrBucketNotFound:     "ErrBucketNotFound",
	ErrBucketExists:       "ErrBucketExists",
	ErrBucketNameRequired: "ErrBucketNameRequired",
	ErrKeyRequired:        "ErrKeyRequired",
	ErrKeyTooLarge:        "ErrKeyTooLarge",
	ErrValueTooLarge:      "ErrValueTooLarge",
	ErrIncompatibleValue:  "ErrIncompatibleValue",
	ErrBlockNotFound:      "ErrBlockNotFound",
	ErrBlockExists:        "ErrBlockExists",
	ErrBlockRegionInvalid: "ErrBlockRegionInvalid",
	ErrDriverSpecific:     "ErrDriverSpecific",
}

func (e ErrorCode) String() string{
	if s := errorCodeStrings[e];s!=""{
		return s
	}
	return fmt.Sprintf("Unknown ErrorCode (%d)", int(e))
}

type Error struct {
	ErrorCode	ErrorCode
	Description	string
	Err	error
}

func (e Error) Error() string{
	if e.Err != nil{
		return e.Description + ": " + e.Err.Error()
	}
	return e.Description
}

func makeError(c ErrorCode,desc string,err error) Error{
	return Error{c,desc,err}
}