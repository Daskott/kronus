package database

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Daskott/kronus/server/auth"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	DEFAULT_PROBE_CRON_DAY    = "3"
	DEFAULT_PROBE_CRON_HOUR   = "18"
	DEFAULT_PROBE_CRON_MINUTE = "0"
)

// At 18:00 every Wednesday
var DEFAULT_PROBE_CRON_EXPRESSION = fmt.Sprintf(
	"%v %v * * %v", DEFAULT_PROBE_CRON_MINUTE, DEFAULT_PROBE_CRON_HOUR, DEFAULT_PROBE_CRON_DAY)

// Maps day of the week to it's numeric equivalent e.g. "sun": "0", "mon": "1" ...
var CRON_DAY_MAPPINGS = map[string]string{
	"sun": "0", "mon": "1", "tue": "2", "wed": "3", "thu": "4", "fri": "5", "sat": "5",
}

var db *gorm.DB

func init() {
	var err error
	dsn := "root:password@tcp(127.0.0.1:3306)/kronus?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			LogLevel:                  logger.Silent,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)})
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
	Fails       int
	Name        string
	Handler     string
	Args        string
	LastError   string
	Claimed     bool `gorm:"default:false"`
	JobStatusID uint
}

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

type Contact struct {
	BaseModel
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	PhoneNumber        string `json:"phone_number" validate:"required,e164" gorm:"not null;unique"`
	Email              string `json:"email" validate:"required,email" gorm:"unique"`
	IsEmergencyContact bool
	UserID             uint             `gorm:"not null"`
	EmergencyProbes    []EmergencyProbe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Role struct {
	BaseModel
	Name  string `json:"name"`
	Users []User `json:"users,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

type ProbeSetting struct {
	BaseModel
	UserID         uint   `gorm:"not null;unique"`
	Active         bool   `gorm:"default:false"`
	CronExpression string `gorm:"not null"`
}

type Probe struct {
	BaseModel
	Response        string
	RetryCount      int
	EmergencyProbe  EmergencyProbe
	LocationLatLong string
	UserID          uint `gorm:"not null"`
	ProbeStatusID   uint
}

type EmergencyProbe struct {
	BaseModel
	ContactID    uint
	ProbeID      uint
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

	user.ProbeSettings = &ProbeSetting{CronExpression: DEFAULT_PROBE_CRON_EXPRESSION}
	return db.Create(user).Error
}

func UpdateUser(id interface{}, data map[string]interface{}) error {
	if data["password"] != nil {
		passwordHash, err := auth.HashPassword(data["password"].(string))
		if err != nil {
			return err
		}
		data["password"] = passwordHash
	}

	return db.Model(&User{}).Where("id = ?", id).Select(
		"FirstName",
		"LastName",
		"PhoneNumber",
		"Password",
	).Updates(data).Error
}

func DeleteUser(id interface{}) error {
	return db.Delete(&User{}, id).Error
}

func FindUserBy(field string, value interface{}) (*User, error) {
	user := User{}
	err := db.Select(
		"ID",
		"FirstName",
		"LastName",
		"PhoneNumber",
		"Email",
		"RoleID",
	).First(&user, fmt.Sprintf("%v = ?", field), value).Error
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

func FindRole(name string) (*Role, error) {
	role := Role{}
	err := db.Select("ID", "Name").First(&role, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return &role, nil
}

func EmergencyContact(userID interface{}) (*Contact, error) {
	contact := Contact{}

	err := db.First(&contact, "user_id = ? AND is_emergency_contact = true", userID).Error
	if err != nil {
		return nil, err
	}

	return &contact, nil
}

func IsAdmin(user *User) (bool, error) {
	if user.RoleID == 0 {
		return false, nil
	}

	adminRole, err := FindRole("admin")
	if err != nil {
		return false, err
	}

	return adminRole.ID == user.RoleID, nil
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

func SetProbeStatus(probeID interface{}, status string) error {
	probeStatus := ProbeStatus{}

	err := db.First(&probeStatus, &ProbeStatus{Name: status}).Error
	if err != nil {
		return err
	}

	err = db.Model(&Probe{}).Where("id = ?", probeID).Update("probe_status_id", probeStatus.ID).Error
	if err != nil {
		return err
	}

	return nil
}

func ProbesByStatus(status string) ([]Probe, error) {
	probes := []Probe{}

	err := db.Joins(
		"INNER JOIN probe_statuses ON probe_statuses.id = probes.probe_status_id AND probe_statuses.name = ?", status).
		Find(&probes).Error

	if err != nil {
		return nil, err
	}

	return probes, err
}

func FindProbeStatus(name string) (*ProbeStatus, error) {
	probeStatus := ProbeStatus{}
	err := db.Select("ID", "Name").First(&probeStatus, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return &probeStatus, nil
}

func LastProbeForUser(userID interface{}) (*Probe, error) {
	probe := Probe{}
	err := db.Last(&probe).Where("userID = ?", userID).Error
	if err != nil {
		return nil, err
	}

	return &probe, nil
}

func FindProbe(id interface{}) (*Probe, error) {
	probe := Probe{}
	err := db.First(&probe, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &probe, nil
}

func CreateProbe(userID interface{}) error {
	currentTime := time.Now()
	pendingProbeStatus := ProbeStatus{}
	err := db.Find(&pendingProbeStatus).Where(&ProbeStatus{Name: "pending"}).Error
	if err != nil {
		return err
	}

	return db.Model(&Probe{}).Create(map[string]interface{}{
		"user_id":         userID,
		"probe_status_id": pendingProbeStatus.ID,
		"created_at":      currentTime,
		"updated_at":      currentTime,
	}).Error
}

func CreateEmergencyProbe(probeID, contactID interface{}) error {
	currentTime := time.Now()
	return db.Model(&EmergencyProbe{}).Create(map[string]interface{}{
		"probe_id":   probeID,
		"contact_id": contactID,
		"created_at": currentTime,
		"updated_at": currentTime,
	}).Error
}

func UpdateProbSettings(userID interface{}, data map[string]interface{}) error {
	return db.Model(&ProbeSetting{}).Where("user_id = ? ", userID).Updates(data).Error
}

func CreateJob(name string, handler string, args string) error {
	enqueuedJobStatus := ProbeStatus{}
	err := db.Find(&enqueuedJobStatus).Where(&JobStatus{Name: "enqueued"}).Error
	if err != nil {
		return err
	}

	return db.Create(&Job{Name: name, Handler: handler, Args: args, JobStatusID: enqueuedJobStatus.ID}).Error
}

func CreateUniqueJobByName(name string, handler string, args string) error {
	queuedJobStatuses := []JobStatus{}
	err := db.Find(&queuedJobStatuses).Where("name IN ('enqueued', 'in-progress')").Error
	if err != nil {
		return err
	}

	// Check to see if a job with a similar name is either enqueued or in-progress
	// if one exists, return 'duplicate' error
	statusIDs := []uint{queuedJobStatuses[0].ID, queuedJobStatuses[1].ID}
	results := db.Where("name = ? AND job_status_id IN ?", name, statusIDs).First(&Job{})

	if results.Error != nil && !errors.Is(results.Error, gorm.ErrRecordNotFound) {
		return err
	}

	if results.RowsAffected > 0 {
		return fmt.Errorf("duplicate job isn't enqueued")
	}

	var enqueuedJobStatus JobStatus
	for _, jobStatus := range queuedJobStatuses {
		if jobStatus.Name == "enqueued" {
			enqueuedJobStatus = jobStatus
			break
		}
	}

	return db.Create(&Job{Name: name, Handler: handler, Args: args, JobStatusID: enqueuedJobStatus.ID}).Error
}

func LastJob(status string, claimed bool) (*Job, error) {
	enqueuedJobStatus := JobStatus{}
	err := db.Find(&enqueuedJobStatus).Where(&JobStatus{Name: status}).Error
	if err != nil {
		return nil, err
	}

	job := Job{}
	err = db.Last(&job, Job{JobStatusID: enqueuedJobStatus.ID, Claimed: claimed}).Error
	if err != nil {
		return nil, err
	}

	return &job, nil
}

func ClaimJob(jobID uint) (bool, error) {
	res := db.Model(&Job{}).Where("id = ? AND claimed = ?", jobID, false).Update("claimed", true)

	if res.Error != nil {
		return false, res.Error
	}

	return res.RowsAffected > 0, nil
}

func UpdateJobStatus(jobID uint, status string) error {
	jobStatus := JobStatus{}
	err := db.Find(&jobStatus).Where(&JobStatus{Name: status}).Error
	if err != nil {
		return err
	}

	return db.Model(&Job{}).Where("ID = ?", jobID).Update("JobStatusID", jobStatus.ID).Error
}

func UpdateJob(jobID uint, data map[string]interface{}) error {
	return db.Table("jobs").Where("id = ?", jobID).Updates(data).Error
}

func FindJobStatus(name string) (*JobStatus, error) {
	jobStatus := JobStatus{}
	err := db.Select("ID", "Name").First(&jobStatus, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return &jobStatus, nil
}

func Save(value interface{}) error {
	return db.Save(value).Error
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
		db.Create(&[]ProbeStatus{{Name: "pending"}, {Name: "good"}, {Name: "bad"}, {Name: "unavailable"}, {Name: "cancelled"}})
	}

	//Insert seed data for JobStatus
	if err := db.First(&JobStatus{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		fmt.Println("Inserting seed data into 'JobStatus'")
		db.Create(&[]JobStatus{{Name: "enqueued"}, {Name: "in-progress"}, {Name: "successful"}, {Name: "failed"}, {Name: "dead"}})
	}

	//Insert seed data for Role
	if err := db.First(&Role{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		fmt.Println("Inserting seed data into 'Role'")
		db.Create(&[]Role{{Name: "admin"}, {Name: "basic"}})
	}
}
