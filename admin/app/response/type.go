package response

type Responses interface {
	SetCode(int32)
	SetMsg(string)
	SetData(interface{})
	SetSuccess(bool)
	Clone() Responses
}
