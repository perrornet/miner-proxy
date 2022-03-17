package common

// 分页条件
type PageWhereOrder struct {
	Order string
	Where string
	Value []interface{}
}

type Navigation struct {
	ID   string `json:"id"`
	Pid  string `json:"pid"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Ico  string `json:"ico"`
}
