package skill

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
)

type Skill struct {
	SkillID             string    `json:"skill_id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	Category             string    `json:"category"`
	RiskLevel            string    `json:"risk_level"`
	RequiredTrustMin     float64   `json:"required_trust_min"`
	RequiredProficiency  int       `json:"required_proficiency"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type AgentSkill struct {
	AgentID          string     `json:"agent_id"`
	SkillID          string     `json:"skill_id"`
	SkillName        string     `json:"skill_name,omitempty"`
	Proficiency      int        `json:"proficiency"`
	Verified         bool       `json:"verified"`
	VerifiedBy       string     `json:"verified_by,omitempty"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	EndorsementsCount int       `json:"endorsements_count"`
	AcquiredAt       time.Time  `json:"acquired_at"`
}

type SkillTrustScore struct {
	AgentID   string    `json:"agent_id"`
	SkillID   string    `json:"skill_id"`
	SkillName string    `json:"skill_name,omitempty"`
	TrustScore float64  `json:"trust_score"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Endorsement struct {
	EndorsementID string    `json:"endorsement_id"`
	AgentID       string    `json:"agent_id"`
	SkillID       string    `json:"skill_id"`
	EndorserType  string    `json:"endorser_type"`
	EndorserID    string    `json:"endorser_id"`
	Comment       string    `json:"comment,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type SkillService struct {
	q  database.Querier
	db *database.DB
}

func NewSkillService(db *database.DB) *SkillService {
	var q database.Querier
	if db != nil && db.Pool != nil {
		q = &database.PoolQuerier{Pool: db.Pool}
	}
	return &SkillService{db: db, q: q}
}

func NewSkillServiceWithQuerier(q database.Querier) *SkillService {
	return &SkillService{q: q}
}

func (s *SkillService) CreateSkill(ctx context.Context, name, description, category, riskLevel string, requiredTrustMin float64, requiredProficiency int) (*Skill, error) {
	if name == "" {
		return nil, fmt.Errorf("skill name is required")
	}
	if riskLevel == "" {
		riskLevel = "medium"
	}
	if category == "" {
		category = "general"
	}
	if requiredTrustMin == 0 {
		requiredTrustMin = 0.5
	}
	if requiredProficiency == 0 {
		requiredProficiency = 1
	}

	id := uuid.New()
	var createdAt time.Time
	err := s.q.QueryRow(ctx,
		`INSERT INTO skills (skill_id, name, description, category, risk_level, required_trust_min, required_proficiency)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at`,
		id, name, description, category, riskLevel, requiredTrustMin, requiredProficiency,
	).Scan(&createdAt)
	if err != nil {
		return nil, fmt.Errorf("create skill: %w", err)
	}

	return &Skill{
		SkillID:            id.String(),
		Name:               name,
		Description:        description,
		Category:            category,
		RiskLevel:           riskLevel,
		RequiredTrustMin:    requiredTrustMin,
		RequiredProficiency: requiredProficiency,
		CreatedAt:           createdAt,
		UpdatedAt:           createdAt,
	}, nil
}

func (s *SkillService) GetSkill(ctx context.Context, skillID string) (*Skill, error) {
	var sk Skill
	err := s.q.QueryRow(ctx,
		`SELECT skill_id, name, description, category, risk_level, required_trust_min, required_proficiency, created_at, updated_at
		 FROM skills WHERE skill_id = $1`,
		skillID,
	).Scan(&sk.SkillID, &sk.Name, &sk.Description, &sk.Category, &sk.RiskLevel, &sk.RequiredTrustMin, &sk.RequiredProficiency, &sk.CreatedAt, &sk.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("skill not found: %w", err)
	}
	return &sk, nil
}

func (s *SkillService) GetSkillByName(ctx context.Context, name string) (*Skill, error) {
	var sk Skill
	err := s.q.QueryRow(ctx,
		`SELECT skill_id, name, description, category, risk_level, required_trust_min, required_proficiency, created_at, updated_at
		 FROM skills WHERE name = $1`,
		name,
	).Scan(&sk.SkillID, &sk.Name, &sk.Description, &sk.Category, &sk.RiskLevel, &sk.RequiredTrustMin, &sk.RequiredProficiency, &sk.CreatedAt, &sk.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("skill not found: %w", err)
	}
	return &sk, nil
}

func (s *SkillService) ListSkills(ctx context.Context, category string) ([]Skill, error) {
	var rows database.Rows
	var err error
	if category != "" {
		rows, err = s.q.Query(ctx,
			`SELECT skill_id, name, description, category, risk_level, required_trust_min, required_proficiency, created_at, updated_at
			 FROM skills WHERE category = $1 ORDER BY name`, category)
	} else {
		rows, err = s.q.Query(ctx,
			`SELECT skill_id, name, description, category, risk_level, required_trust_min, required_proficiency, created_at, updated_at
			 FROM skills ORDER BY name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var sk Skill
		if err := rows.Scan(&sk.SkillID, &sk.Name, &sk.Description, &sk.Category, &sk.RiskLevel, &sk.RequiredTrustMin, &sk.RequiredProficiency, &sk.CreatedAt, &sk.UpdatedAt); err != nil {
			continue
		}
		skills = append(skills, sk)
	}
	return skills, nil
}

func (s *SkillService) UpdateSkill(ctx context.Context, skillID, description, category, riskLevel string, requiredTrustMin float64, requiredProficiency int) (*Skill, error) {
	_, err := s.q.Exec(ctx,
		`UPDATE skills SET description = $1, category = $2, risk_level = $3, required_trust_min = $4, required_proficiency = $5, updated_at = NOW()
		 WHERE skill_id = $6`,
		description, category, riskLevel, requiredTrustMin, requiredProficiency, skillID,
	)
	if err != nil {
		return nil, fmt.Errorf("update skill: %w", err)
	}
	return s.GetSkill(ctx, skillID)
}

func (s *SkillService) DeleteSkill(ctx context.Context, skillID string) error {
	_, err := s.q.Exec(ctx, `DELETE FROM skills WHERE skill_id = $1`, skillID)
	if err != nil {
		return fmt.Errorf("delete skill: %w", err)
	}
	return nil
}

func (s *SkillService) AssignSkill(ctx context.Context, agentID, skillID string, proficiency int) (*AgentSkill, error) {
	if proficiency < 1 {
		proficiency = 1
	}
	if proficiency > 5 {
		proficiency = 5
	}

	var acquiredAt time.Time
	err := s.q.QueryRow(ctx,
		`INSERT INTO agent_skills (agent_id, skill_id, proficiency)
		 VALUES ($1::uuid, $2::uuid, $3)
		 ON CONFLICT (agent_id, skill_id) DO UPDATE SET proficiency = $3, acquired_at = NOW()
		 RETURNING acquired_at`,
		agentID, skillID, proficiency,
	).Scan(&acquiredAt)
	if err != nil {
		return nil, fmt.Errorf("assign skill: %w", err)
	}

	return &AgentSkill{
		AgentID:     agentID,
		SkillID:     skillID,
		Proficiency: proficiency,
		AcquiredAt:  acquiredAt,
	}, nil
}

func (s *SkillService) RemoveSkill(ctx context.Context, agentID, skillID string) error {
	_, err := s.q.Exec(ctx,
		`DELETE FROM agent_skills WHERE agent_id = $1::uuid AND skill_id = $2::uuid`,
		agentID, skillID,
	)
	if err != nil {
		return fmt.Errorf("remove skill: %w", err)
	}
	return nil
}

func (s *SkillService) ListAgentSkills(ctx context.Context, agentID string) ([]AgentSkill, error) {
	rows, err := s.q.Query(ctx,
		`SELECT als.agent_id, als.skill_id, s.name, als.proficiency, als.verified, COALESCE(als.verified_by, ''), als.verified_at, als.endorsements_count, als.acquired_at
		 FROM agent_skills als JOIN skills s ON s.skill_id = als.skill_id
		 WHERE als.agent_id = $1::uuid ORDER BY s.name`,
		agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AgentSkill
	for rows.Next() {
		var as AgentSkill
		if err := rows.Scan(&as.AgentID, &as.SkillID, &as.SkillName, &as.Proficiency, &as.Verified, &as.VerifiedBy, &as.VerifiedAt, &as.EndorsementsCount, &as.AcquiredAt); err != nil {
			continue
		}
		result = append(result, as)
	}
	return result, nil
}

func (s *SkillService) VerifySkill(ctx context.Context, agentID, skillID, verifiedBy string) (*AgentSkill, error) {
	_, err := s.q.Exec(ctx,
		`UPDATE agent_skills SET verified = true, verified_by = $1, verified_at = NOW() WHERE agent_id = $2::uuid AND skill_id = $3::uuid`,
		verifiedBy, agentID, skillID,
	)
	if err != nil {
		return nil, fmt.Errorf("verify skill: %w", err)
	}

	skills, err := s.ListAgentSkills(ctx, agentID)
	if err != nil {
		return nil, err
	}
	for i := range skills {
		if skills[i].SkillID == skillID {
			return &skills[i], nil
		}
	}
	return nil, fmt.Errorf("skill not found after verification")
}

func (s *SkillService) EndorseSkill(ctx context.Context, agentID, skillID, endorserType, endorserID, comment string) (*Endorsement, error) {
	validTypes := map[string]bool{"human": true, "agent": true, "ptv": true}
	if !validTypes[endorserType] {
		return nil, fmt.Errorf("invalid endorser_type: %s (must be human, agent, or ptv)", endorserType)
	}

	id := uuid.New()
	var createdAt time.Time
	err := s.q.QueryRow(ctx,
		`INSERT INTO skill_endorsements (endorsement_id, agent_id, skill_id, endorser_type, endorser_id, comment)
		 VALUES ($1, $2::uuid, $3::uuid, $4, $5, $6) RETURNING created_at`,
		id, agentID, skillID, endorserType, endorserID, comment,
	).Scan(&createdAt)
	if err != nil {
		return nil, fmt.Errorf("endorse skill: %w", err)
	}

	return &Endorsement{
		EndorsementID: id.String(),
		AgentID:       agentID,
		SkillID:       skillID,
		EndorserType:  endorserType,
		EndorserID:    endorserID,
		Comment:       comment,
		CreatedAt:     createdAt,
	}, nil
}

func (s *SkillService) ListEndorsements(ctx context.Context, agentID, skillID string) ([]Endorsement, error) {
	var rows database.Rows
	var err error
	if skillID != "" {
		rows, err = s.q.Query(ctx,
			`SELECT endorsement_id, agent_id, skill_id, endorser_type, endorser_id, comment, created_at
			 FROM skill_endorsements WHERE agent_id = $1::uuid AND skill_id = $2::uuid ORDER BY created_at DESC`,
			agentID, skillID,
		)
	} else {
		rows, err = s.q.Query(ctx,
			`SELECT endorsement_id, agent_id, skill_id, endorser_type, endorser_id, comment, created_at
			 FROM skill_endorsements WHERE agent_id = $1::uuid ORDER BY created_at DESC`,
			agentID,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endorsements []Endorsement
	for rows.Next() {
		var e Endorsement
		if err := rows.Scan(&e.EndorsementID, &e.AgentID, &e.SkillID, &e.EndorserType, &e.EndorserID, &e.Comment, &e.CreatedAt); err != nil {
			continue
		}
		endorsements = append(endorsements, e)
	}
	return endorsements, nil
}

func (s *SkillService) GetAgentSkillTrust(ctx context.Context, agentID string) ([]SkillTrustScore, error) {
	rows, err := s.q.Query(ctx,
		`SELECT sts.agent_id, sts.skill_id, s.name, sts.trust_score, sts.updated_at
		 FROM skill_trust_scores sts JOIN skills s ON s.skill_id = sts.skill_id
		 WHERE sts.agent_id = $1::uuid ORDER BY s.name`,
		agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []SkillTrustScore
	for rows.Next() {
		var sc SkillTrustScore
		if err := rows.Scan(&sc.AgentID, &sc.SkillID, &sc.SkillName, &sc.TrustScore, &sc.UpdatedAt); err != nil {
			continue
		}
		scores = append(scores, sc)
	}
	return scores, nil
}

func (s *SkillService) GetSkillTrust(ctx context.Context, agentID, skillID string) (float64, error) {
	var trustScore float64
	err := s.q.QueryRow(ctx,
		`SELECT trust_score FROM skill_trust_scores WHERE agent_id = $1::uuid AND skill_id = $2::uuid`,
		agentID, skillID,
	).Scan(&trustScore)
	if err != nil {
		return 0, fmt.Errorf("skill trust score not found: %w", err)
	}
	return trustScore, nil
}

func (s *SkillService) UpdateSkillTrust(ctx context.Context, agentID, skillID string, delta float64) (float64, error) {
	var newScore float64

	currentScore := 1.0
	s.q.QueryRow(ctx,
		`SELECT trust_score FROM skill_trust_scores WHERE agent_id = $1::uuid AND skill_id = $2::uuid`,
		agentID, skillID,
	).Scan(&currentScore)

	newScore = currentScore + delta
	if newScore < 0 {
		newScore = 0
	}
	if newScore > 1 {
		newScore = 1
	}

	_, err := s.q.Exec(ctx,
		`INSERT INTO skill_trust_scores (agent_id, skill_id, trust_score, updated_at)
		 VALUES ($1::uuid, $2::uuid, $3, NOW())
		 ON CONFLICT (agent_id, skill_id) DO UPDATE SET trust_score = $3, updated_at = NOW()`,
		agentID, skillID, newScore,
	)
	if err != nil {
		return 0, fmt.Errorf("update skill trust: %w", err)
	}
	return newScore, nil
}

func (s *SkillService) CheckSkillAuthorization(ctx context.Context, agentID, action string) (bool, []string, float64, error) {
	rows, err := s.q.Query(ctx,
		`SELECT s.name, s.required_proficiency, s.required_trust_min, als.proficiency, als.verified,
		 COALESCE(sts.trust_score, -1) as skill_trust
		 FROM skills s
		 JOIN agent_skills als ON als.skill_id = s.skill_id AND als.agent_id = $1::uuid
		 LEFT JOIN skill_trust_scores sts ON sts.agent_id = $1::uuid AND sts.skill_id = s.skill_id
		 WHERE s.name = $2`,
		agentID, action,
	)
	if err != nil {
		return false, nil, 0, nil
	}
	defer rows.Close()

	for rows.Next() {
		var skillName string
		var reqProf int
		var reqTrust float64
		var proficiency int
		var verified bool
		var skillTrust float64

		if err := rows.Scan(&skillName, &reqProf, &reqTrust, &proficiency, &verified, &skillTrust); err != nil {
			continue
		}

		if proficiency < reqProf {
			return false, []string{fmt.Sprintf("proficiency %d < required %d for skill %s", proficiency, reqProf, skillName)}, skillTrust, nil
		}

		effectiveTrust := skillTrust
		if skillTrust < 0 {
			var globalTrust float64
			err := s.q.QueryRow(ctx, `SELECT trust_score FROM agents WHERE agent_id = $1::uuid`, agentID).Scan(&globalTrust)
			if err != nil {
				effectiveTrust = 1.0
			} else {
				effectiveTrust = globalTrust
			}
		}

		if effectiveTrust < reqTrust {
			return false, []string{fmt.Sprintf("trust %.4f < required %.4f for skill %s", effectiveTrust, reqTrust, skillName)}, effectiveTrust, nil
		}

		return true, nil, effectiveTrust, nil
	}

	return true, nil, 1.0, nil
}

func (s *SkillService) FindMissingSkills(ctx context.Context, agentID string, requiredSkills []string) ([]string, error) {
	if len(requiredSkills) == 0 {
		return nil, nil
	}

	rows, err := s.q.Query(ctx,
		`SELECT s.name FROM skills s WHERE s.name = ANY($1)
		 AND s.skill_id NOT IN (
			 SELECT als.skill_id FROM agent_skills als WHERE als.agent_id = $2::uuid
		 )`,
		requiredSkills, agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var missing []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		missing = append(missing, name)
	}
	return missing, nil
}