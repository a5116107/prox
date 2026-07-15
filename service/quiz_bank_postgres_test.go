package service

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestQuizDrawConcurrentPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("QUIZ_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("QUIZ_POSTGRES_TEST_DSN is not set")
	}

	adminDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := "quiz_concurrency_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	require.NoError(t, adminDB.Exec(`CREATE SCHEMA "`+schema+`"`).Error)
	t.Cleanup(func() {
		_ = adminDB.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
	})

	db, err := gorm.Open(postgres.Open(postgresTestDSNWithSchema(t, dsn, schema)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.GameRound{},
		&model.GameEntry{},
		&model.QuizBank{},
		&model.QuizCategory{},
		&model.QuizQuestion{},
		&model.QuizBankBinding{},
		&model.QuizQuestionDraw{},
		&model.AgentBudgetPool{},
		&model.AgentBudgetTransaction{},
		&model.OpsFundAccount{},
		&model.OpsFundLedger{},
	))

	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldSQLite, oldMySQL, oldPostgreSQL := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	oldAgent := *operation_setting.GetAgentSetting()
	oldMembership := *operation_setting.GetMembershipRiskSetting()
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgreSQL
		*operation_setting.GetAgentSetting() = oldAgent
		*operation_setting.GetMembershipRiskSetting() = oldMembership
	})
	model.DB, model.LOG_DB = db, db
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = false, false, true
	operation_setting.GetAgentSetting().SiteID = "quiz-test"
	operation_setting.GetMembershipRiskSetting().Enabled = false

	require.NoError(t, db.Create(&model.User{
		Id: quizTestUserID, Username: "quiz_pg_user", Password: "not-used-in-test", Status: 1,
	}).Error)
	seedQuizBank(t, "postgres-concurrent", "only one open round is allowed")

	const workers = 24
	start := make(chan struct{})
	results := make(chan int, workers)
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			<-start
			payload := quizTestPayload(fmt.Sprintf("postgres-worker-%d", worker))
			result, drawErr := quizQuestionDraw(quizTestAction(fmt.Sprintf("postgres-worker-%d", worker)), payload)
			if drawErr != nil {
				errors <- drawErr
				return
			}
			drawID, ok := result["draw_id"].(int)
			if !ok {
				if floatID, floatOK := result["draw_id"].(float64); floatOK {
					drawID, ok = int(floatID), true
				}
			}
			if !ok || drawID <= 0 {
				errors <- fmt.Errorf("worker %d received invalid draw_id: %#v", worker, result["draw_id"])
				return
			}
			results <- drawID
		}(index)
	}
	close(start)
	wait.Wait()
	close(results)
	close(errors)

	for drawErr := range errors {
		require.NoError(t, drawErr)
	}
	firstDrawID := 0
	count := 0
	for drawID := range results {
		if firstDrawID == 0 {
			firstDrawID = drawID
		}
		require.Equal(t, firstDrawID, drawID)
		count++
	}
	require.Equal(t, workers, count)

	var openDraws, openRounds int64
	require.NoError(t, db.Model(&model.QuizQuestionDraw{}).
		Where("site_id = ? AND status = ?", "quiz-test", "open").Count(&openDraws).Error)
	require.NoError(t, db.Model(&model.GameRound{}).
		Where("site_id = ? AND game_code = ? AND status = ?", "quiz-test", "quiz", "open").Count(&openRounds).Error)
	require.Equal(t, int64(1), openDraws)
	require.Equal(t, int64(1), openRounds)
}

func postgresTestDSNWithSchema(t *testing.T, dsn, schema string) string {
	t.Helper()
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return strings.TrimSpace(dsn) + " search_path=" + schema
}
