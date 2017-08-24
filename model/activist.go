package model

import (
	"database/sql"
	"time"

	"github.com/pkg/errors"

	"github.com/jmoiron/sqlx"
)

/** Constant and Variable Definitions */

const selectUserBaseQuery string = `
SELECT
  id,
  name,
  email,
  chapter,
  phone,
  location,
  facebook
FROM activists
`

const selectUserExtraBaseQuery string = `
SELECT
  a.id,
  a.name,
  email,
  chapter,
  phone,
  location,
  facebook,
  activist_level,
  exclude_from_leaderboard,
  core_staff,
  global_team_member,
  liberation_pledge,
  MIN(e.date) AS first_event,
  MAX(e.date) AS last_event,
  COUNT(e.id) as total_events
FROM activists a

LEFT JOIN event_attendance ea
  ON ea.activist_id = a.id
 
LEFT JOIN events e
  ON ea.event_id = e.id
`

const descOrder int = 2
const ascOrder int = 1

/** Type Definitions */

type User struct {
	ID               int            `db:"id"`
	Name             string         `db:"name"`
	Email            string         `db:"email"`
	Chapter          string         `db:"chapter"`
	Phone            string         `db:"phone"`
	Location         sql.NullString `db:"location"`
	Facebook         string         `db:"facebook"`
	LiberationPledge int            `db:"liberation_pledge"`
}

type UserEventData struct {
	FirstEvent  *time.Time `db:"first_event"`
	LastEvent   *time.Time `db:"last_event"`
	TotalEvents int        `db:"total_events"`
	Status      string
}

type UserMembershipData struct {
	CoreStaff              int    `db:"core_staff"`
	ExcludeFromLeaderboard int    `db:"exclude_from_leaderboard"`
	GlobalTeamMember       int    `db:"global_team_member"`
	ActivistLevel          string `db:"activist_level"`
}

type UserExtra struct {
	User
	UserEventData
	UserMembershipData
}

type UserJSON struct {
	ID                     int    `json:"id"`
	Name                   string `json:"name"`
	Email                  string `json:"email"`
	Chapter                string `json:"chapter"`
	Phone                  string `json:"phone"`
	Location               string `json:"location"`
	Facebook               string `json:"facebook"`
	FirstEvent             string `json:"first_event"`
	LastEvent              string `json:"last_event"`
	TotalEvents            int    `json:"total_events"`
	Status                 string `json:"status"`
	Core                   int    `json:"core_staff"`
	ExcludeFromLeaderboard int    `json:"exclude_from_leaderboard"`
	LiberationPledge       int    `json:"liberation_pledge"`
	GlobalTeamMember       int    `json:"global_team_member"`
	ActivistLevel          string `json:"activist_level"`
}

type UserOptionsJSON struct {
	Name  string `json:"name"`
	Limit int    `json:"limit"`
	Order int    `json:"order"`
}

/** Functions and Methods */

func GetUsersJSON(db *sqlx.DB) ([]UserJSON, error) {
	return getUsersJSON(db, 0)
}

func GetUserJSON(db *sqlx.DB, userID int) (UserJSON, error) {
	users, err := getUsersJSON(db, userID)
	if err != nil {
		return UserJSON{}, err
	}
	return users[0], nil
}

func GetUserRangeJSON(db *sqlx.DB, userOptions UserOptionsJSON) ([]UserJSON, error) {
	// Check that order matches one of the defined order constants
	if userOptions.Order != descOrder && userOptions.Order != ascOrder {
		return nil, errors.New("User Range order must be ascending or descending")
	}
	users, err := getUserRange(db, userOptions)
	if err != nil {
		return nil, err
	}
	return buildUserJSONArray(users), nil
}

func getUsersJSON(db *sqlx.DB, userID int) ([]UserJSON, error) {
	users, err := GetUsersExtra(db, userID)
	if err != nil {
		return nil, err
	}
	usersJSON := buildUserJSONArray(users)
	return usersJSON, nil
}

func buildUserJSONArray(users []UserExtra) []UserJSON {
	var usersJSON []UserJSON
	for _, u := range users {
		firstEvent := ""
		if u.UserEventData.FirstEvent != nil {
			firstEvent = u.UserEventData.FirstEvent.Format(EventDateLayout)
		}
		lastEvent := ""
		if u.UserEventData.LastEvent != nil {
			lastEvent = u.UserEventData.LastEvent.Format(EventDateLayout)
		}
		location := ""
		if u.User.Location.Valid {
			location = u.User.Location.String
		}

		usersJSON = append(usersJSON, UserJSON{
			ID:            u.User.ID,
			Name:          u.User.Name,
			Email:         u.User.Email,
			Chapter:       u.User.Chapter,
			Phone:         u.User.Phone,
			Location:      location,
			Facebook:      u.User.Facebook,
			ActivistLevel: u.ActivistLevel,
			FirstEvent:    firstEvent,
			LastEvent:     lastEvent,
			TotalEvents:   u.UserEventData.TotalEvents,
			Status:        u.Status,
			Core:          u.CoreStaff,
			ExcludeFromLeaderboard: u.ExcludeFromLeaderboard,
			LiberationPledge:       u.LiberationPledge,
			GlobalTeamMember:       u.GlobalTeamMember,
		})
	}

	return usersJSON
}

func GetUser(db *sqlx.DB, name string) (User, error) {
	users, err := getUsers(db, name)
	if err != nil {
		return User{}, err
	} else if len(users) == 0 {
		return User{}, errors.New("Could not find any users")
	} else if len(users) > 1 {
		return User{}, errors.New("Found too many users")
	}
	return users[0], nil
}

func GetUsers(db *sqlx.DB) ([]User, error) {
	return getUsers(db, "")
}

func getUsers(db *sqlx.DB, name string) ([]User, error) {
	var queryArgs []interface{}
	query := selectUserBaseQuery

	if name != "" {
		query += " WHERE name = ? "
		queryArgs = append(queryArgs, name)
	}

	query += " ORDER BY name "

	var users []User
	if err := db.Select(&users, query, queryArgs...); err != nil {
		return nil, errors.Wrapf(err, "failed to get users for %s", name)
	}

	return users, nil
}

func GetUsersExtra(db *sqlx.DB, userID int) ([]UserExtra, error) {
	query := selectUserExtraBaseQuery

	var queryArgs []interface{}

	if userID != 0 {
		// retrieve specific user rather than all users
		query += " WHERE a.id = ? "
		queryArgs = append(queryArgs, userID)
	}
	query += " GROUP BY a.id "

	var users []UserExtra
	if err := db.Select(&users, query, queryArgs...); err != nil {
		return nil, errors.Wrapf(err, "failed to get users extra for uid %d", userID)
	}

	for i := 0; i < len(users); i++ {
		u := users[i]
		users[i].Status = getStatus(u.FirstEvent, u.LastEvent, u.TotalEvents)
	}

	return users, nil
}

func getUserRange(db *sqlx.DB, userOptions UserOptionsJSON) ([]UserExtra, error) {
	query := selectUserExtraBaseQuery
	name := userOptions.Name
	order := userOptions.Order
	limit := userOptions.Limit
	var queryArgs []interface{}

	if name != "" {
		if order == descOrder {
			query += " WHERE a.name < ? "
		} else {
			query += " WHERE a.name > ? "
		}
		queryArgs = append(queryArgs, name)
	}

	query += " GROUP BY a.name ORDER BY a.name "
	if order == descOrder {
		query += "desc "
	}

	if limit > 0 {
		query += " LIMIT ? "
		queryArgs = append(queryArgs, limit)
	}

	var users []UserExtra
	if err := db.Select(&users, query, queryArgs...); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve %d users before/after %s", limit, name)
	}

	return users, nil
}

func (u User) GetUserEventData(db *sqlx.DB) (UserEventData, error) {
	query := `
SELECT
  MIN(e.date) AS first_event,
  MAX(e.date) AS last_event,
  COUNT(*) as total_events
FROM events e
JOIN event_attendance
  ON event_attendance.event_id = e.id
WHERE
  event_attendance.activist_id = ?
`
	var data UserEventData
	if err := db.Get(&data, query, u.ID); err != nil {
		return UserEventData{}, errors.Wrap(err, "failed to get user event data")
	}
	return data, nil
}

func GetOrCreateUser(db *sqlx.DB, name string) (User, error) {
	user, err := GetUser(db, name)
	if err == nil {
		// We got a valid user, return them.
		return user, nil
	}

	// There was an error, so try inserting the user first.
	// Wrap in transaction to avoid issue where a new user
	// is inserted successfully, but we are unable to retrieve
	// the new user, which will leave database in inconsistent state

	tx, err := db.Beginx()
	if err != nil {
		return User{}, errors.Wrap(err, "Failed to create transaction")
	}

	_, err = tx.Exec("INSERT INTO activists (name) VALUES (?)", name)
	if err != nil {
		tx.Rollback()
		return User{}, errors.Wrapf(err, "failed to insert user %s", name)
	}

	query := selectUserBaseQuery + " WHERE name = ? "

	var newUser User
	err = tx.Get(&newUser, query, name)

	if err != nil {
		tx.Rollback()
		return User{}, errors.Wrapf(err, "failed to get new user %s", name)
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return User{}, errors.Wrapf(err, "failed to commit user %s", name)
	}

	return newUser, nil
}

func UpdateActivistData(db *sqlx.DB, user UserExtra) (int, error) {
	_, err := db.NamedExec(`UPDATE activists
SET
  name = :name,
  email = :email,
  chapter = :chapter,
  phone = :phone,
  location = :location,
  facebook = :facebook,
  activist_level = :activist_level,
  exclude_from_leaderboard = :exclude_from_leaderboard,
  core_staff = :core_staff,
  global_team_member = :global_team_member,
  liberation_pledge = :liberation_pledge
WHERE
id = :id`, user)

	if err != nil {
		return 0, errors.Wrap(err, "failed to update activist data")
	}
	return user.ID, nil
}
