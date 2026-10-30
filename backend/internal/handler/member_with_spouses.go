package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MemberWithSpouseList 成员含配偶列表
type MemberWithSpouseList struct {
	Member
	Spouses []Spouse `json:"spouses"`
}

// ListWithSpouses 返回所有成员，每个成员附带配偶列表
// GET /api/members/with-spouses
func (h *MemberHandler) ListWithSpouses(c *gin.Context) {
	bcond, bargs := BranchScope(c, h.DB, "branch_id")
	mq := `SELECT id, generation, branch, branch_id, surname, given_name, name, zi, hao, generation_word,
		rank, gender, father_id, mother_id, mother_spouse_id, clan_type, record_type, foster_father_id,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era, death_gregorian, death_lunar, death_ganzhi, death_era,
		lifespan, native_place, current_place, title, occupation, education, biography, burial_place, portrait, married_out, former_name, phone, dna_haploid, dna_diploid, dna_mito
		FROM members`
	if bcond != "" {
		mq += " WHERE " + bcond
	}
	mq += " ORDER BY generation, id"
	rows, err := h.DB.Query(mq, bargs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	members := []MemberWithSpouseList{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Generation, &m.Branch, &m.BranchID, &m.Surname, &m.GivenName, &m.Name, &m.Zi, &m.Hao,
			&m.GenerationWord, &m.Rank, &m.Gender, &m.FatherID, &m.MotherID, &m.MotherSpouseID,
			&m.ClanType, &m.RecordType, &m.FosterFatherID, &m.BirthCalendar, &m.BirthGregorian, &m.BirthLunar, &m.BirthGanzhi, &m.BirthEra,
			&m.DeathGregorian, &m.DeathLunar, &m.DeathGanzhi, &m.DeathEra, &m.Lifespan,
			&m.NativePlace, &m.CurrentPlace, &m.Title, &m.Occupation, &m.Education, &m.Biography, &m.BurialPlace, &m.Portrait, &m.MarriedOut, &m.FormerName, &m.Phone, &m.DNAHaploid, &m.DNADiploid, &m.DNAMito); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 查询配偶
		spRows, _ := h.DB.Query(`SELECT id, member_id, surname, given_name, name, type, family,
			birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era,
			death_gregorian, death_lunar, death_ganzhi, death_era, COALESCE(occupation,''), COALESCE(education,''), COALESCE(biography,''), COALESCE(remark,''), COALESCE(portrait,''), COALESCE(phone,''), COALESCE(dna_haploid,''), COALESCE(dna_diploid,''), COALESCE(dna_mito,'')
			FROM spouses WHERE member_id = ? ORDER BY
			CASE type WHEN '正配' THEN 1 WHEN '继配' THEN 2 WHEN '侧室' THEN 3 ELSE 4 END, id`, m.ID)
		spouses := []Spouse{}
		for spRows.Next() {
			var s Spouse
			spRows.Scan(&s.ID, &s.MemberID, &s.Surname, &s.GivenName, &s.Name, &s.Type, &s.Family,
				&s.BirthCalendar, &s.BirthGregorian, &s.BirthLunar, &s.BirthGanzhi, &s.BirthEra,
				&s.DeathGregorian, &s.DeathLunar, &s.DeathGanzhi, &s.DeathEra,
				&s.Occupation, &s.Education, &s.Biography, &s.Remark, &s.Portrait, &s.Phone, &s.DNAHaploid, &s.DNADiploid, &s.DNAMito)
			spouses = append(spouses, s)
		}
		spRows.Close()

		members = append(members, MemberWithSpouseList{Member: m, Spouses: spouses})
	}

	// 解密敏感字段 + 隐私脱敏
	viewerID := FindMemberByName(h.DB, c.GetString("username"))
	flags := memberFlagsMap(h.DB)
	bioSet := bioFlagSet(h.DB)
	bioSpouseSet := bioSpouseFlagSet(h.DB)
	for i := range members {
		if v, ok := flags[members[i].ID]; ok {
			members[i].Member.NoHeir = v[0]
			members[i].Member.LostContact = v[1]
		}
		members[i].Member.HasBio = bioSet[members[i].ID]
		for j := range members[i].Spouses {
			members[i].Spouses[j].HasBio = bioSpouseSet[members[i].Spouses[j].ID]
		}
		members[i].Member.DecryptSensitive(h.EncKey)
		decryptSpousesInPlace(members[i].Spouses, h.EncKey)
		SanitizeMember(&members[i].Member, &members[i].Spouses, c.GetString("role"), viewerID, c.GetString("branch"), h.DB)
	}
	c.JSON(http.StatusOK, members)
}
