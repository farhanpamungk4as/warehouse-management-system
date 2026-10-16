package httpx

import "github.com/gin-gonic/gin"

type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
type Failure struct {
	Status  int          `json:"-"`
	Message string       `json:"message"`
	Errors  []FieldError `json:"errors"`
}

func NewFailure(status int, message, reason string) *Failure {
	return &Failure{Status: status, Message: message, Errors: []FieldError{{Field: "request", Reason: reason}}}
}
func (f *Failure) Error() string { return f.Message }

func Success(c *gin.Context, status int, message string, data any) {
	c.JSON(status, struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    any    `json:"data"`
	}{true, message, data})
}

func SuccessWithMeta(c *gin.Context, status int, message string, data, meta any) {
	c.JSON(status, struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    any    `json:"data"`
		Meta    any    `json:"meta"`
	}{true, message, data, meta})
}

func WriteFailure(c *gin.Context, f *Failure) {
	errs := f.Errors
	if errs == nil {
		errs = []FieldError{}
	}
	c.JSON(f.Status, struct {
		Success bool         `json:"success"`
		Message string       `json:"message"`
		Errors  []FieldError `json:"errors"`
	}{false, f.Message, errs})
}

func Reject(c *gin.Context, err error) { _ = c.Error(err); c.Abort() }
