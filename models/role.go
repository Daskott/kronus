package models

const (
	ADMIN_USER_ROLE = "admin"
	BASIC_USER_ROLE = "basic"
)

type Role struct {
	BaseModel
	Name  string `json:"name"`
	Users []User `json:"users,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func FindRole(name string) (*Role, error) {
	role := Role{}
	err := db.Select("id", "name").First(&role, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return &role, nil
}
