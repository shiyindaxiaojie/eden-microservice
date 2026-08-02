package auth

// User represents a console user.
type User struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	Nickname       string `json:"nickname"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	Remark         string `json:"remark"`
	Role           string `json:"role"`
	IsBuiltIn      bool   `json:"is_builtin"`
	GuideCompleted bool   `json:"guide_completed"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}
