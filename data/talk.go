package data

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-hclog"
	"github.com/jinzhu/gorm"
)

type Talk struct {
	// gorm.Model
	ID                uint       `json:"id" gorm:"primary_key;auto_increment"`
	Title             string     `json:"title" gorm:"not null"`
	DurationInMinutes uint       `json:"durationInMinutes" gorm:"not null"`
	Language          string     `json:"language" gorm:"not null"`
	Level             TalkLevel  `json:"level" gorm:"not null"`
	Persons           []Person   `json:"persons,omitempty" gorm:"many2many:talks_at;association_autoupdate:false"`
	Topics            []Topic    `json:"topics,omitempty" gorm:"many2many:talk_topic;association_autoupdate:false"`
	TalkDates         []TalkDate `json:"talkDates,omitempty" gorm:"foreignkey:TalkID;association_autoupdate:false"`
}

type TalkLevel string

const (
	BeginnerLevel TalkLevel = "beginner"
	AdvancedLevel           = "advanced"
	ExpertLevel             = "expert"
)

type TalkStore interface {
	GetTalks() ([]*Talk, error)
	GetTalkByID(id uint) (*Talk, error)
	UpdateTalk(id uint, talk *Talk) (*Talk, error)
	AddTalk(talk *Talk) (*Talk, error)
	DeleteTalkByID(id uint) error
	GetTalksByEventID(eventID uint) ([]*Talk, error)
	GetTalksByPersonID(personID uint) ([]*Talk, error)
}

type TalkDBStore struct {
	*gorm.DB
	validate *validator.Validate
	log hclog.Logger
}

type TalkNotFoundError struct {
	Cause error
}

func (e TalkNotFoundError) Error() string { return "Talk not found! Cause: " + e.Cause.Error() }
func (e TalkNotFoundError) Unwrap() error { return e.Cause }

func NewTalkDBStore(db *gorm.DB, log hclog.Logger) *TalkDBStore {
	return &TalkDBStore{db, validator.New(), log}
}

func (db *TalkDBStore) GetTalks() ([]*Talk, error) {
	db.log.Debug("Getting all talks...")

	var talks []*Talk
	if err := db.
		Preload("Persons").
		Preload("Persons.Organization").
		Preload("Topics").
		Preload("Topics.Children").
		Preload("TalkDates").
		Preload("TalkDates.Room").
		Preload("TalkDates.Event").
		Find(&talks).Error; err != nil {
		db.log.Error("Error getting all talks", "err", err)
		return []*Talk{}, err
	}

	db.log.Debug("Returning talks", "talks", spew.Sprintf("%+v", talks))
	return talks, nil
}

func (db *TalkDBStore) GetTalkByID(id uint) (*Talk, error) {
	db.log.Debug("Getting talk by id...", "id", id)

	var talk Talk
	if err := db.Preload("Persons").
		Preload("Persons.Organization").
		Preload("Topics").
		Preload("Topics.Children").
		Preload("TalkDates").
		Preload("TalkDates.Room").
		Preload("TalkDates.Event").
		First(&talk, id).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			db.log.Error("Talk not found by id", "id", id)
			return nil, &TalkNotFoundError{err}
		} else {
			db.log.Error("Unexpected error getting talk by id", "err", err)
			return nil, err
		}
	}

	db.log.Debug("Returning talk", "talk", hclog.Fmt("%+v", talk))
	return &talk, nil
}

func (db *TalkDBStore) UpdateTalk(id uint, talk *Talk) (*Talk, error) {
	db.log.Debug("Updating talk...", "talk", hclog.Fmt("%+v", talk))

	err := db.validate.Struct(talk)
	if err != nil {
		db.log.Error("Error validating talk", "err", err)
		return nil, err
	}

	if err := db.Model(&Talk{}).Where("id = ?", id).Take(&Talk{}).Update(talk).First(&talk, id).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			db.log.Error("Talk to be updated not found", "talk", hclog.Fmt("%+v", talk))
			return nil, &TalkNotFoundError{err}
		} else {
			db.log.Error("Unexpected error updating talk", "err", err)
			return nil, err
		}
	}

	db.log.Debug("Successfully updated talk", "talk", hclog.Fmt("%+v", talk))
	return db.GetTalkByID(talk.ID)
}

func (db *TalkDBStore) AddTalk(talk *Talk) (*Talk, error) {
	db.log.Debug("Adding talk...", "talk", hclog.Fmt("%+v", talk))

	err := db.validate.Struct(talk)
	if err != nil {
		db.log.Error("Error validating talk", "err", err)
		return nil, err
	}

	if err := db.Create(&talk).Error; err != nil {
		db.log.Error("Unexpected error creating talk", "err", err)
		return nil, err
	}

	db.log.Debug("Successfully added talk", "talk", hclog.Fmt("%+v", talk))
	return db.GetTalkByID(talk.ID)
}

func (db *TalkDBStore) DeleteTalkByID(id uint) error {
	db.log.Debug("Deleting talk by id...", "id", id)

	if err := db.Model(&Talk{}).Where("id = ?", id).Take(&Talk{}).Delete(&Talk{}).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			db.log.Error("Talk not found by id", "id", id)
			return &TalkNotFoundError{err}
		} else {
			db.log.Error("Unexpected error deleting talk", "err", err)
			return err
		}
	}

	db.log.Debug("Successfully deleted talk")
	return nil
}

func (db *TalkDBStore) GetTalksByEventID(eventID uint) ([]*Talk, error) {
	db.log.Debug("Getting talks by event id...", "eventID", eventID)

	var talks []*Talk
	if err := db.
		Table("talk").
		Preload("Persons").
		Preload("Persons.Organization").
		Preload("Topics").
		Preload("Topics.Children").
		Preload("TalkDates").
		Preload("TalkDates.Event").
		Preload("TalkDates.Room").
		Where("id IN ?", db.Table("talk_date").Select("talk_id").Where("event_id = ?", eventID).SubQuery()).
		Find(&talks).Error; err != nil {
		db.log.Error("Error getting talks", "err", err)
		return []*Talk{}, err
	}

	db.log.Debug("Returning talks", "talks", spew.Sprintf("%+v", talks))
	return talks, nil
}

func (db *TalkDBStore) GetTalksByPersonID(personID uint) ([]*Talk, error) {
	db.log.Debug("Getting talks by person id...", "personID", personID)

	var talks []*Talk
	if err := db.
		Preload("Persons").
		Preload("Persons.Organization").
		Preload("Topics").
		Where("id IN ?", db.Table("talks_at").Select("talk_id").Where("person_id = ?", personID).SubQuery()).
		Find(&talks).Error; err != nil {
		db.log.Error("Error getting talks", "err", err)
		return []*Talk{}, err
	}

	db.log.Debug("Returning talks", "talks", spew.Sprintf("%+v", talks))
	return talks, nil
}
