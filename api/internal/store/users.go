package store

import (
	"ai-workbench-api/internal/model"
	"time"

	"github.com/sirupsen/logrus"
)

func CreateUser(u *model.User) error {
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	if mysqlOK {
		_, err := db.Exec(`INSERT INTO users (id,username,password_hash,role,must_change_pwd,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
			u.ID, u.Username, u.PasswordHash, u.Role, u.MustChangePwd, u.CreatedAt, u.UpdatedAt)
		return err
	}
	return nil
}

func GetUserByUsername(username string) *model.User {
	if !mysqlOK {
		return nil
	}
	var u model.User
	err := db.QueryRow(`SELECT id,username,password_hash,role,must_change_pwd,created_at,updated_at FROM users WHERE username=?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.MustChangePwd, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil
	}
	return &u
}

func UpdateUserPassword(id, hash string) {
	if mysqlOK {
		_, _ = db.Exec(`UPDATE users SET password_hash=?,must_change_pwd=0,updated_at=? WHERE id=?`, hash, time.Now(), id)
	}
}

func UserCount() int {
	if !mysqlOK {
		return 0
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		logrus.Warnf("user count: %v", err)
		return 0
	}
	return count
}
