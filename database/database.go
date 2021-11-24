package database

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var db *gorm.DB

func init() {
	var err error
	dsn := "daskott:password@tcp(127.0.0.1:3306)/kronus?charset=utf8mb4&parseTime=True&loc=Local"
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

type BaseUserModel struct {
	FirstName   string
	LastName    string
	PhoneNumber string `gorm:"not null;unique"`
}

type Job struct {
	BaseModel
	RetryCount  int
	Task        string
	JobStatusID uint
}

type User struct {
	BaseModel
	BaseUserModel
	Email         string       `gorm:"not null;unique"`
	PasswordHash  string       `gorm:"not null"`
	ProbeSettings ProbeSetting `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Contacts      []Contact    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Probes        []Probe      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Contact struct {
	BaseModel
	BaseUserModel
	IsEmergencyContact bool
	UserID             uint             `gorm:"not null"`
	Email              string           `gorm:"unique"`
	EmergencyProbes    []EmergencyProbe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
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

func AutoMigrate() {
	// Migrate the schema
	db.AutoMigrate(&ProbeStatus{})
	db.AutoMigrate(&JobStatus{})

	db.AutoMigrate(&Job{})
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
}
