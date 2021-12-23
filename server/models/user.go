package models

import (
	"errors"
	"fmt"

	"github.com/Daskott/kronus/server/auth"
	"gorm.io/gorm"
)

var (
	allFieldsExceptPassword = []string{"id",
		"first_name",
		"last_name",
		"phone_number",
		"email",
		"role_id",
		"created_at",
		"updated_at",
	}

	updatableFields = []string{"first_name",
		"last_name",
		"phone_number",
		"password",
	}
)

type User struct {
	BaseModel
	FirstName     string        `json:"first_name" validate:"required"`
	LastName      string        `json:"last_name" validate:"required"`
	PhoneNumber   string        `json:"phone_number" validate:"required,e164" gorm:"not null;unique"`
	Email         string        `json:"email" validate:"required,email" gorm:"not null;unique"`
	Password      string        `json:"password,omitempty" validate:"required,password" gorm:"not null"`
	RoleID        uint          `json:"role_id" gorm:"null"`
	ProbeSettings *ProbeSetting `json:"probe_settings,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Contacts      []Contact     `json:"contacts,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Probes        []Probe       `json:"probes,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// DisableProbe turns off probe for user & cancels all pending probes
func (user *User) DisableLivlinessProbe() error {
	err := user.UpdateProbSettings(map[string]interface{}{"active": false})
	if err != nil {
		return err
	}

	return user.CancelAllPendingProbes()
}

func (user *User) CancelAllPendingProbes() error {
	probes := []Probe{}

	// Fetch all pending probes for user
	err := db.Joins(
		"INNER JOIN probe_statuses ON probe_statuses.id = probes.probe_status_id AND probe_statuses.name = ?", PENDING_PROBE).
		Where("user_id = ?", user.ID).Find(&probes).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// If pending probes exist, set status for all of them to 'cancelled'
	if len(probes) > 0 {
		cancelledStatus := ProbeStatus{}
		err := db.Where(&ProbeStatus{Name: CANCELLED_PROBE}).Find(&cancelledStatus).Error
		if err != nil {
			return err
		}

		probeIDs := []uint{}
		for _, probe := range probes {
			probeIDs = append(probeIDs, probe.ID)
		}

		return db.Table("probes").
			Where("id IN ?", probeIDs).Update("probe_status_id", cancelledStatus.ID).Error
	}

	return nil
}

func (user *User) Update(data map[string]interface{}) error {
	if data["password"] != nil {
		passwordHash, err := auth.HashPassword(data["password"].(string))
		if err != nil {
			return err
		}
		data["password"] = passwordHash
	}

	return db.Model(&User{}).Where("id = ?", user.ID).Select(updatableFields).Updates(data).Error
}

func (user *User) UpdateProbSettings(data map[string]interface{}) error {
	return db.Model(&ProbeSetting{}).Where("user_id = ? ", user.ID).Updates(data).Error
}

func (user *User) IsAdmin() (bool, error) {
	if user.RoleID == 0 {
		return false, nil
	}

	adminRole, err := FindRole("admin")
	if err != nil {
		return false, err
	}

	return adminRole.ID == user.RoleID, nil
}

func (user *User) AddContact(contact *Contact) error {
	contact.UserID = user.ID
	return db.Create(contact).Error
}

func (user *User) LoadContacts() error {
	// TODO: Add pagination
	return db.Limit(500).Find(&user.Contacts, "user_id = ?", user.ID).Error
}

func (user *User) UpdateContact(contactID string, data map[string]interface{}) error {
	return db.Table("contacts").Where("id = ? AND user_id = ?", contactID, user.ID).Updates(data).Error
}

func (user *User) DeleteContact(id interface{}) error {
	return db.Where("user_id = ?", user.ID).Delete(&Contact{}, id).Error
}

func (user *User) EmergencyContact() (*Contact, error) {
	contact := Contact{}

	err := db.Where("user_id = ? AND is_emergency_contact = true", user.ID).First(&contact).Error
	if err != nil {
		return nil, err
	}

	return &contact, nil
}

func (user *User) LastProbe() (*Probe, error) {
	probe := Probe{}
	err := db.Where("user_id = ?", user.ID).Last(&probe).Error
	if err != nil {
		return nil, err
	}

	return &probe, nil
}

func (user *User) IsProbeEnabled() (bool, error) {
	pbSettings, err := FindProbeSettings(user.ID)
	if err != nil {
		return false, nil
	}

	return pbSettings.Active, nil
}

func FindProbeSettings(userID interface{}) (*ProbeSetting, error) {
	probeSetting := ProbeSetting{}
	err := db.First(&probeSetting, "user_id = ?", userID).Error
	if err != nil {
		return nil, err
	}

	return &probeSetting, nil
}

func FindUserBy(field string, value interface{}) (*User, error) {
	user := User{}
	err := db.Select(allFieldsExceptPassword).First(&user, fmt.Sprintf("%v = ?", field), value).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func FindUserWithProbeSettiings(userID interface{}) (*User, error) {
	user := User{}
	err := db.Preload("ProbeSettings").Select(allFieldsExceptPassword).First(&user, userID).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func FindUserPassword(email string) (string, error) {
	user := &User{}
	err := db.Select("Password").First(user, "email = ?", email).Error

	if err != nil {
		return "", err
	}
	return user.Password, nil
}

func CreateUser(user *User) error {
	passwordHash, err := auth.HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = passwordHash

	user.ProbeSettings = &ProbeSetting{CronExpression: DEFAULT_PROBE_CRON_EXPRESSION}
	return db.Create(user).Error
}

func DeleteUser(id interface{}) error {
	return db.Delete(&User{}, id).Error
}

func AtLeastOneUserExists() (bool, error) {
	err := db.First(&User{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func UsersWithActiveProbe() ([]User, error) {
	users := []User{}

	// Get users with 'active' probe set & include their probe_settings
	err := db.Preload("ProbeSettings").Joins(
		"INNER JOIN probe_settings ON probe_settings.user_id = users.id AND probe_settings.active = true").
		Find(&users).Error

	if err != nil {
		return nil, err
	}

	return users, nil
}
