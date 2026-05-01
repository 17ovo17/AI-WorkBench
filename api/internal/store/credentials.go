package store

import "ai-workbench-api/internal/model"

// SaveCredential persists a credential to memory and MySQL.
func SaveCredential(c *model.Credential) {
	mu.Lock()
	credentials[c.ID] = c
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`REPLACE INTO credentials (id,name,protocol,username,password,ssh_key,port,remark) VALUES (?,?,?,?,?,?,?,?)`, c.ID, c.Name, c.Protocol, c.Username, c.Password, c.SSHKey, c.Port, c.Remark)
	}
}

// DeleteCredential removes a credential by id.
func DeleteCredential(id string) {
	mu.Lock()
	delete(credentials, id)
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM credentials WHERE id=?`, id)
	}
}

// GetCredential retrieves a credential by id.
func GetCredential(id string) (*model.Credential, bool) {
	if mysqlOK {
		row := db.QueryRow(`SELECT id,name,protocol,username,password,ssh_key,port,remark FROM credentials WHERE id=?`, id)
		c := model.Credential{}
		if row.Scan(&c.ID, &c.Name, &c.Protocol, &c.Username, &c.Password, &c.SSHKey, &c.Port, &c.Remark) == nil {
			return &c, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	c, ok := credentials[id]
	return c, ok
}

// ListCredentials returns all credentials with sensitive fields masked.
func ListCredentials() []*model.Credential {
	out := []*model.Credential{}
	if mysqlOK {
		rows, err := db.Query(`SELECT id,name,protocol,username,password,ssh_key,port,remark FROM credentials ORDER BY name`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				c := model.Credential{}
				_ = rows.Scan(&c.ID, &c.Name, &c.Protocol, &c.Username, &c.Password, &c.SSHKey, &c.Port, &c.Remark)
				maskCredential(&c)
				out = append(out, &c)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, c := range credentials {
		safe := *c
		maskCredential(&safe)
		out = append(out, &safe)
	}
	return out
}

func maskCredential(c *model.Credential) {
	if c.Password != "" {
		c.Password = "******"
	}
	if c.SSHKey != "" {
		c.SSHKey = "******"
	}
}
