package handler

import (
	"database/sql"

	"familytree/internal/crypto"
)

// ============================================================================
// 成员敏感字段加解密（透明层：在 handler → DB 之间拦截）
// ============================================================================

// EncryptSensitive 加密成员的敏感字段（入库前调用）
func (m *Member) EncryptSensitive(encKey string) error {
	if encKey == "" {
		return nil
	}
	if m.Phone != "" {
		enc, err := crypto.Encrypt(m.Phone, encKey)
		if err != nil {
			return err
		}
		m.Phone = enc
	}
	if m.DNAHaploid != "" {
		enc, err := crypto.Encrypt(m.DNAHaploid, encKey)
		if err != nil {
			return err
		}
		m.DNAHaploid = enc
	}
	if m.DNADiploid != "" {
		enc, err := crypto.Encrypt(m.DNADiploid, encKey)
		if err != nil {
			return err
		}
		m.DNADiploid = enc
	}
	if m.DNAMito != "" {
		enc, err := crypto.Encrypt(m.DNAMito, encKey)
		if err != nil {
			return err
		}
		m.DNAMito = enc
	}
	return nil
}

// DecryptSensitive 解密成员的敏感字段（出库后调用）
func (m *Member) DecryptSensitive(encKey string) error {
	if encKey == "" {
		return nil
	}
	if m.Phone != "" {
		plain, err := crypto.Decrypt(m.Phone, encKey)
		if err != nil {
			return err
		}
		m.Phone = plain
	}
	if m.DNAHaploid != "" {
		plain, err := crypto.Decrypt(m.DNAHaploid, encKey)
		if err != nil {
			return err
		}
		m.DNAHaploid = plain
	}
	if m.DNADiploid != "" {
		plain, err := crypto.Decrypt(m.DNADiploid, encKey)
		if err != nil {
			return err
		}
		m.DNADiploid = plain
	}
	if m.DNAMito != "" {
		plain, err := crypto.Decrypt(m.DNAMito, encKey)
		if err != nil {
			return err
		}
		m.DNAMito = plain
	}
	return nil
}

// DecryptValue 解密单个敏感字段值（失败时返回原值，用于保留库中原值）
func DecryptValue(enc, encKey string) string {
	if enc == "" || encKey == "" {
		return enc
	}
	if plain, err := crypto.Decrypt(enc, encKey); err == nil {
		return plain
	}
	return enc
}

// DecryptMembers decrypts sensitive fields for a slice of members
func DecryptMembers(members []Member, encKey string) error {
	for i := range members {
		if err := members[i].DecryptSensitive(encKey); err != nil {
			return err
		}
	}
	return nil
}

// ============================================================================
// 加密密钥管理：从 settings 表读取，不存在则创建
// ============================================================================
const encKeySetting = "enc_key"

// GetOrCreateEncKey 从 settings 表获取或创建 AES 加密密钥
func GetOrCreateEncKey(db *sql.DB) (string, error) {
	var key string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, encKeySetting).Scan(&key)
	if err == nil && key != "" {
		return key, nil
	}
	// 不存在则生成
	key = crypto.GenerateKey()
	_, err = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)`, encKeySetting, key)
	if err != nil {
		return "", err
	}
	// 迁移：加密已存在的明文手机号（首次启动场景）
	encryptExistingPhones(db, key)
	return key, nil
}

// encryptExistingPhones 加密数据库里已存在的明文手机号（迁移用）
func encryptExistingPhones(db *sql.DB, encKey string) {
	rows, err := db.Query(`SELECT id, phone FROM members WHERE phone != ''`)
	if err != nil {
		return
	}
	defer rows.Close()
	type rec struct {
		id    int64
		phone string
	}
	var updates []rec
	for rows.Next() {
		var r rec
		rows.Scan(&r.id, &r.phone)
		// 跳过已加密的（hex 密文长度 >> 11，明文手机号 7-15 位）
		if len(r.phone) > 20 {
			continue
		}
		encrypted, err := crypto.Encrypt(r.phone, encKey)
		if err == nil {
			updates = append(updates, rec{r.id, encrypted})
		}
	}
	for _, u := range updates {
		db.Exec(`UPDATE members SET phone = ? WHERE id = ?`, u.phone, u.id)
	}
}

// ============================================================================
// 配偶敏感字段加解密（与成员同规则：手机号 + DNA 单/二倍体/线粒体）
// ============================================================================

// EncryptSensitive 加密配偶敏感字段（入库前调用）
func (s *Spouse) EncryptSensitive(encKey string) error {
	if encKey == "" {
		return nil
	}
	if s.Phone != "" {
		enc, err := crypto.Encrypt(s.Phone, encKey)
		if err != nil {
			return err
		}
		s.Phone = enc
	}
	for _, f := range []*string{&s.DNAHaploid, &s.DNADiploid, &s.DNAMito} {
		if *f != "" {
			enc, err := crypto.Encrypt(*f, encKey)
			if err != nil {
				return err
			}
			*f = enc
		}
	}
	return nil
}

// DecryptSensitive 解密配偶敏感字段（出库后调用，容忍个别字段解密失败）
func (s *Spouse) DecryptSensitive(encKey string) error {
	if encKey == "" {
		return nil
	}
	var firstErr error
	if s.Phone != "" {
		if plain, err := crypto.Decrypt(s.Phone, encKey); err == nil {
			s.Phone = plain
		} else {
			firstErr = err
		}
	}
	for _, f := range []*string{&s.DNAHaploid, &s.DNADiploid, &s.DNAMito} {
		if *f != "" {
			if plain, err := crypto.Decrypt(*f, encKey); err == nil {
				*f = plain
			} else if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// decryptSpousesInPlace 解密配偶切片中的敏感字段（读取接口调用）
func decryptSpousesInPlace(list []Spouse, encKey string) {
	for i := range list {
		_ = list[i].DecryptSensitive(encKey)
	}
}