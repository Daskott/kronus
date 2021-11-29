package database

import (
	"errors"
	"fmt"
	"time"

	"github.com/Daskott/kronus/server/auth"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var db *gorm.DB

func init() {
	var err error
	dsn := "root:password@tcp(127.0.0.1:3306)/kronus?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
}

type BaseModel struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Job struct {
	BaseModel
	RetryCount  int
	Task        string
	JobStatusID uint
}

type User struct {
	BaseModel
	FirstName     string        `json:"first_name"`
	LastName      string        `json:"last_name"`
	PhoneNumber   string        `json:"phone_number" validate:"required,e164" gorm:"not null;unique"`
	Email         string        `json:"email" validate:"required,email" gorm:"not null;unique"`
	Password      string        `json:"password,omitempty" validate:"required,password" gorm:"not null"`
	RoleID        uint          `json:"role_id" gorm:"null"`
	ProbeSettings *ProbeSetting `json:"probe_settings,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Contacts      []Contact     `json:"contacts,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Probes        []Probe       `json:"probes,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Contact struct {
	BaseModel
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	PhoneNumber        string `json:"phone_number" validate:"required,phone_number" gorm:"not null;unique"`
	Email              string `json:"email" validate:"required,email" gorm:"unique"`
	IsEmergencyContact bool
	UserID             uint             `gorm:"not null"`
	EmergencyProbes    []EmergencyProbe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Role struct {
	BaseModel
	Name  string `json:"name"`
	Users []User `json:"probes,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

type ProbeSetting struct {
	BaseModel
	UserID uint `gorm:"not null;unique"`
	Active bool `gorm:"default:false"`
}

type Probe struct {
	BaseModel
	Response         string
	RetryCount       int
	LocationLatLong  string
	UserID           uint `gorm:"not null"`
	ProbeStatusID    uint
	EmergencyProbeID uint
}

type EmergencyProbe struct {
	BaseModel
	ContactID    uint
	Probe        Probe
	Acknowledged bool `gorm:"default:false"`
}

type ProbeStatus struct {
	BaseModel
	Name   string
	Probes []Probe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

type JobStatus struct {
	BaseModel
	Name string
	Jobs []Job `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func CreateUser(user *User) error {
	passwordHash, err := auth.HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = passwordHash

	return db.Create(user).Error
}

func UpdateUser(id interface{}, data map[string]interface{}) error {
	user := User{}

	err := db.Select("ID", "FirstName", "LastName").First(&user, id).Error
	if err != nil {
		return err
	}

	if data["password"] != nil {
		passwordHash, err := auth.HashPassword(user.Password)
		if err != nil {
			return err
		}
		data["password"] = passwordHash
	}

	return db.Model(&user).Select(
		"FirstName",
		"LastName",
		"PhoneNumber",
		"Password",
	).Updates(data).Error
}

func DeleteUser(id interface{}) error {
	return db.Delete(&User{}, id).Error
}

func FindUserBy(user *User, field string, value interface{}) error {
	return db.Select(
		"ID",
		"FirstName",
		"LastName",
		"PhoneNumber",
		"Email",
		"RoleID",
	).First(user, fmt.Sprintf("%v = ?", field), value).Error
}

func FindUserPassword(email string) (string, error) {
	user := &User{}
	err := db.Select("Password").First(user, "email = ?", email).Error

	if err != nil {
		return "", err
	}
	return user.Password, nil
}

func AtLeastOneUserExists() bool {
	if err := db.First(&User{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	return true
}

func FindRole(name string) (*Role, error) {
	role := Role{}
	err := db.Select("ID", "Name").First(&role, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return &role, nil
}

func IsAdmin(user User) (bool, error) {
	if user.RoleID == 0 {
		return false, nil
	}

	adminRole, err := FindRole("admin")
	if err != nil {
		return false, err
	}

	return adminRole.ID == user.RoleID, nil
}

func AutoMigrate() {
	// Migrate the schema
	db.AutoMigrate(&ProbeStatus{})
	db.AutoMigrate(&JobStatus{})

	db.AutoMigrate(&Job{})
	db.AutoMigrate(&Role{})
	db.AutoMigrate(&User{})

	db.AutoMigrate(&Probe{})
	db.AutoMigrate(&Contact{})
	db.AutoMigrate(&ProbeSetting{})
	db.AutoMigrate(&EmergencyProbe{})

	//Insert seed data for ProbeStatus
	if err := db.First(&ProbeStatus{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		fmt.Println("Inserting seed data into 'ProbeStatus'")
		db.Create(&[]ProbeStatus{{Name: "pending"}, {Name: "good"}, {Name: "bad"}, {Name: "unavailable"}})
	}

	//Insert seed data for JobStatus
	if err := db.First(&JobStatus{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		fmt.Println("Inserting seed data into 'JobStatus'")
		db.Create(&[]JobStatus{{Name: "pending"}, {Name: "successful"}, {Name: "failed"}})
	}

	//Insert seed data for Role
	if err := db.First(&Role{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		fmt.Println("Inserting seed data into 'Role'")
		db.Create(&[]Role{{Name: "admin"}, {Name: "basic"}})
	}
}
