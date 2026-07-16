package model

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var commonGroupCol string
var commonKeyCol string
var commonTrueVal string
var commonFalseVal string

var logKeyCol string
var logGroupCol string

func initCol() {
	// init common column names
	if common.UsingPostgreSQL {
		commonGroupCol = `"group"`
		commonKeyCol = `"key"`
		commonTrueVal = "true"
		commonFalseVal = "false"
	} else {
		commonGroupCol = "`group`"
		commonKeyCol = "`key`"
		commonTrueVal = "1"
		commonFalseVal = "0"
	}
	if os.Getenv("LOG_SQL_DSN") != "" {
		switch common.LogSqlType {
		case common.DatabaseTypePostgreSQL:
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		default:
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	} else {
		// LOG_SQL_DSN 为空时，日志数据库与主数据库相同
		if common.UsingPostgreSQL {
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		} else {
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	}
	// log sql type and database type
	//common.SysLog("Using Log SQL Type: " + common.LogSqlType)
}

var DB *gorm.DB

var LOG_DB *gorm.DB

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != common.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		common.SysLog("no user exists, create a root user for you: username is root, password is 123456")
		hashedPassword, err := common.Password2Hash("123456")
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
		DB.Create(&rootUser)
	}
	return nil
}

func CheckSetup() {
	setup := GetSetup()
	if setup == nil {
		// No setup record exists, check if we have a root user
		if RootUserExists() {
			common.SysLog("system is not initialized, but root user exists")
			// Create setup record
			newSetup := Setup{
				Version:       common.Version,
				InitializedAt: time.Now().Unix(),
			}
			err := DB.Create(&newSetup).Error
			if err != nil {
				common.SysLog("failed to create setup record: " + err.Error())
			}
			constant.Setup = true
		} else {
			common.SysLog("system is not initialized and no root user exists")
			constant.Setup = false
		}
	} else {
		// Setup record exists, system is initialized
		common.SysLog("system is already initialized at: " + time.Unix(setup.InitializedAt, 0).String())
		constant.Setup = true
	}
}

func chooseDB(envName string, isLog bool) (*gorm.DB, error) {
	defer func() {
		initCol()
	}()
	dsn := os.Getenv(envName)
	if dsn != "" {
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			// Use PostgreSQL
			common.SysLog("using PostgreSQL as database")
			if !isLog {
				common.UsingPostgreSQL = true
			} else {
				common.LogSqlType = common.DatabaseTypePostgreSQL
			}
			return gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true, // disables implicit prepared statement usage
			}), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		if strings.HasPrefix(dsn, "local") {
			common.SysLog("SQL_DSN not set, using SQLite as database")
			if !isLog {
				common.UsingSQLite = true
			} else {
				common.LogSqlType = common.DatabaseTypeSQLite
			}
			return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		// Use MySQL
		common.SysLog("using MySQL as database")
		// check parseTime
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		if !isLog {
			common.UsingMySQL = true
		} else {
			common.LogSqlType = common.DatabaseTypeMySQL
		}
		return gorm.Open(mysql.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
	}
	// Use SQLite
	common.SysLog("SQL_DSN not set, using SQLite as database")
	common.UsingSQLite = true
	return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func InitDB() (err error) {
	db, err := chooseDB("SQL_DSN", false)
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		DB = db
		// MySQL charset/collation startup check: ensure Chinese-capable charset
		if common.UsingMySQL {
			if err := checkMySQLChineseSupport(DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		common.SysLog("database migration started")
		err = migrateDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func InitLogDB() (err error) {
	if os.Getenv("LOG_SQL_DSN") == "" {
		LOG_DB = DB
		return
	}
	db, err := chooseDB("LOG_SQL_DSN", true)
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		LOG_DB = db
		// If log DB is MySQL, also ensure Chinese-capable charset
		if common.LogSqlType == common.DatabaseTypeMySQL {
			if err := checkMySQLChineseSupport(LOG_DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := LOG_DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		common.SysLog("database migration started")
		err = migrateLOGDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func migrateDB() error {
	// Migrate price_amount column from float/double to decimal for existing tables
	migrateSubscriptionPlanPriceAmount()
	// Migrate model_limits column from varchar to text for existing tables
	if err := migrateTokenModelLimitsToText(); err != nil {
		return err
	}
	if err := migrateCheckinLegacyUniqueIndex(); err != nil {
		return err
	}
	if err := migrateUserSiteAccessStateColumns(); err != nil {
		return err
	}

	err := DB.AutoMigrate(
		&Channel{},
		&Token{},
		&User{},
		&UserQuotaReservation{},
		&UserQuotaTransaction{},
		&PasskeyCredential{},
		&Option{},
		&Redemption{},
		&Ability{},
		&Log{},
		&Midjourney{},
		&TopUp{},
		&QuotaData{},
		&Task{},
		&TaskBillingOperation{},
		&Model{},
		&Vendor{},
		&PrefillGroup{},
		&Setup{},
		&TwoFA{},
		&TwoFABackupCode{},
		&Checkin{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionPreConsumeRecord{},
		&SubscriptionQuotaTransaction{},
		&CustomOAuthProvider{},
		&UserOAuthBinding{},
		&CommunityBotReward{},
		&CommunityBotInvite{},
		&CommunityBotMessageStat{},
		&CommunityBotRoomState{},
		&CommunityBotMessageClaim{},
		&ChatMembershipState{},
		&ChatMembershipEvent{},
		&CommunityGateAudit{},
		&CommunityGateTokenFreeze{},
		&AgentConfig{},
		&AgentEvent{},
		&AgentAction{},
		&AgentActionApproval{},
		&AgentBudgetPool{},
		&AgentBudgetTransaction{},
		&OpsFundAccount{},
		&OpsFundLedger{},
		&AgentRiskProfile{},
		&AgentRiskEvent{},
		&AgentMemory{},
		&AgentMemoryCandidate{},
		&AgentMemoryEvent{},
		&AgentMemoryPolicy{},
		&AgentChatMessage{},
		&AgentActivityPlan{},
		&AgentChatBinding{},
		&AgentChatBindCode{},
		&ChatGroup{},
		&GroupGameConfig{},
		&GroupChatOpsConfig{},
		&GroupI18nTemplate{},
		&UserIdentityBinding{},
		&UserSiteAccessState{},
		&UserOpsProfileSnapshot{},
		&InviteCampaign{},
		&InviteLink{},
		&InviteEdge{},
		&InviteEvent{},
		&InviteRewardClaim{},
		&InviteRiskFlag{},
		&GameRound{},
		&GameEntry{},
		&GameSettlement{},
		&GameCommission{},
		&GameJackpotAccount{},
		&GameJackpotLedger{},
		&GroupMetricsDaily{},
		&QuizBank{},
		&QuizCategory{},
		&QuizQuestion{},
		&QuizBankBinding{},
		&QuizQuestionDraw{},
		&OpsReleaseSnapshot{},
		&AdminConfigAudit{},
		&OpsAccountRiskAudit{},
		&OpsAccountRiskAction{},
		&RiskUserControl{},
		&RiskTokenActivation{},
		&OAuthRegisterAttempt{},
		&RiskRequestFingerprint{},
		&AgentTask{},
		&AgentTaskRun{},
		&PerfMetric{},
	)
	if err != nil {
		return err
	}
	if common.UsingSQLite {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
	} else {
		if err := DB.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	return finalizeUserQuotaAccountingMigration()
}

func migrateDBFast() error {

	var wg sync.WaitGroup

	migrations := []struct {
		model interface{}
		name  string
	}{
		{&Channel{}, "Channel"},
		{&Token{}, "Token"},
		{&User{}, "User"},
		{&UserQuotaReservation{}, "UserQuotaReservation"},
		{&UserQuotaTransaction{}, "UserQuotaTransaction"},
		{&PasskeyCredential{}, "PasskeyCredential"},
		{&Option{}, "Option"},
		{&Redemption{}, "Redemption"},
		{&Ability{}, "Ability"},
		{&Log{}, "Log"},
		{&Midjourney{}, "Midjourney"},
		{&TopUp{}, "TopUp"},
		{&QuotaData{}, "QuotaData"},
		{&Task{}, "Task"},
		{&TaskBillingOperation{}, "TaskBillingOperation"},
		{&Model{}, "Model"},
		{&Vendor{}, "Vendor"},
		{&PrefillGroup{}, "PrefillGroup"},
		{&Setup{}, "Setup"},
		{&TwoFA{}, "TwoFA"},
		{&TwoFABackupCode{}, "TwoFABackupCode"},
		{&Checkin{}, "Checkin"},
		{&SubscriptionOrder{}, "SubscriptionOrder"},
		{&UserSubscription{}, "UserSubscription"},
		{&SubscriptionPreConsumeRecord{}, "SubscriptionPreConsumeRecord"},
		{&SubscriptionQuotaTransaction{}, "SubscriptionQuotaTransaction"},
		{&CustomOAuthProvider{}, "CustomOAuthProvider"},
		{&UserOAuthBinding{}, "UserOAuthBinding"},
		{&CommunityBotReward{}, "CommunityBotReward"},
		{&CommunityBotInvite{}, "CommunityBotInvite"},
		{&CommunityBotMessageStat{}, "CommunityBotMessageStat"},
		{&CommunityBotRoomState{}, "CommunityBotRoomState"},
		{&CommunityBotMessageClaim{}, "CommunityBotMessageClaim"},
		{&ChatMembershipState{}, "ChatMembershipState"},
		{&ChatMembershipEvent{}, "ChatMembershipEvent"},
		{&CommunityGateAudit{}, "CommunityGateAudit"},
		{&CommunityGateTokenFreeze{}, "CommunityGateTokenFreeze"},
		{&AgentConfig{}, "AgentConfig"},
		{&AgentEvent{}, "AgentEvent"},
		{&AgentAction{}, "AgentAction"},
		{&AgentActionApproval{}, "AgentActionApproval"},
		{&AgentBudgetPool{}, "AgentBudgetPool"},
		{&AgentBudgetTransaction{}, "AgentBudgetTransaction"},
		{&OpsFundAccount{}, "OpsFundAccount"},
		{&OpsFundLedger{}, "OpsFundLedger"},
		{&ChatGroup{}, "ChatGroup"},
		{&GroupGameConfig{}, "GroupGameConfig"},
		{&GroupChatOpsConfig{}, "GroupChatOpsConfig"},
		{&GroupI18nTemplate{}, "GroupI18nTemplate"},
		{&UserIdentityBinding{}, "UserIdentityBinding"},
		{&UserSiteAccessState{}, "UserSiteAccessState"},
		{&UserOpsProfileSnapshot{}, "UserOpsProfileSnapshot"},
		{&InviteCampaign{}, "InviteCampaign"},
		{&InviteLink{}, "InviteLink"},
		{&InviteEdge{}, "InviteEdge"},
		{&InviteEvent{}, "InviteEvent"},
		{&InviteRewardClaim{}, "InviteRewardClaim"},
		{&InviteRiskFlag{}, "InviteRiskFlag"},
		{&GameRound{}, "GameRound"},
		{&GameEntry{}, "GameEntry"},
		{&GameSettlement{}, "GameSettlement"},
		{&GameCommission{}, "GameCommission"},
		{&GameJackpotAccount{}, "GameJackpotAccount"},
		{&GameJackpotLedger{}, "GameJackpotLedger"},
		{&GroupMetricsDaily{}, "GroupMetricsDaily"},
		{&QuizBank{}, "QuizBank"},
		{&QuizCategory{}, "QuizCategory"},
		{&QuizQuestion{}, "QuizQuestion"},
		{&QuizBankBinding{}, "QuizBankBinding"},
		{&QuizQuestionDraw{}, "QuizQuestionDraw"},
		{&OpsReleaseSnapshot{}, "OpsReleaseSnapshot"},
		{&AdminConfigAudit{}, "AdminConfigAudit"},
		{&OpsAccountRiskAudit{}, "OpsAccountRiskAudit"},
		{&AgentRiskProfile{}, "AgentRiskProfile"},
		{&AgentRiskEvent{}, "AgentRiskEvent"},
		{&AgentMemory{}, "AgentMemory"},
		{&AgentMemoryCandidate{}, "AgentMemoryCandidate"},
		{&AgentMemoryEvent{}, "AgentMemoryEvent"},
		{&AgentMemoryPolicy{}, "AgentMemoryPolicy"},
		{&AgentActivityPlan{}, "AgentActivityPlan"},
		{&AgentChatBinding{}, "AgentChatBinding"},
		{&AgentChatBindCode{}, "AgentChatBindCode"},
		{&RiskUserControl{}, "RiskUserControl"},
		{&RiskTokenActivation{}, "RiskTokenActivation"},
		{&OAuthRegisterAttempt{}, "OAuthRegisterAttempt"},
		{&RiskRequestFingerprint{}, "RiskRequestFingerprint"},
		{&PerfMetric{}, "PerfMetric"},
	}
	// 动态计算migration数量，确保errChan缓冲区足够大
	errChan := make(chan error, len(migrations))

	for _, m := range migrations {
		wg.Add(1)
		go func(model interface{}, name string) {
			defer wg.Done()
			if err := DB.AutoMigrate(model); err != nil {
				errChan <- fmt.Errorf("failed to migrate %s: %v", name, err)
			}
		}(m.model, m.name)
	}

	// Wait for all migrations to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	// This model owns real foreign keys to audit, token, user, and source action.
	// Migrate it after the parallel base-table pass to avoid DDL races.
	if err := DB.AutoMigrate(&OpsAccountRiskAction{}); err != nil {
		return fmt.Errorf("failed to migrate OpsAccountRiskAction: %v", err)
	}
	if err := migrateUsersUsernameLowerUniqueIndex(); err != nil {
		return err
	}
	if common.UsingSQLite {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
	} else {
		if err := DB.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	if err := finalizeUserQuotaAccountingMigration(); err != nil {
		return err
	}
	common.SysLog("database migrated")
	return nil
}

func finalizeUserQuotaAccountingMigration() error {
	const repairID = "conditional-debit-rollout-v1"
	repaired, err := repairAllNegativeUserQuotas(repairID, 10_000)
	if err != nil {
		return fmt.Errorf("failed to repair historical negative user quotas: %w", err)
	}
	if repaired > 0 {
		common.SysLog(fmt.Sprintf("repaired %d historical negative user quota balances", repaired))
	}
	report, err := ReconcileUserQuotaAccounting(time.Now())
	if err != nil {
		return fmt.Errorf("failed to reconcile user quota accounting: %w", err)
	}
	if report.NegativeUsers > 0 || report.OrphanReservations > 0 || report.ReservationLedgerMismatches > 0 {
		return fmt.Errorf(
			"user quota accounting reconciliation failed: negative=%d orphan=%d ledger_mismatch=%d",
			report.NegativeUsers, report.OrphanReservations, report.ReservationLedgerMismatches,
		)
	}
	return nil
}

func migrateUsersUsernameLowerUniqueIndex() error {
	if DB == nil || !DB.Migrator().HasTable(&User{}) {
		return nil
	}
	if common.UsingPostgreSQL {
		var duplicateCount int64
		if err := DB.Raw(`
			SELECT COUNT(*) FROM (
				SELECT LOWER(TRIM(username)) AS username_key
				FROM users
				WHERE username IS NOT NULL AND TRIM(username) <> ''
				GROUP BY LOWER(TRIM(username))
				HAVING COUNT(*) > 1
			) dup
		`).Scan(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return fmt.Errorf("users username lower-case duplicates detected: %d groups; run scripts/audit_oauth_identity.sql before enabling users_username_lower_key", duplicateCount)
		}
		return DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS users_username_lower_key ON users (LOWER(TRIM(username)))`).Error
	}
	if common.UsingSQLite {
		return DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS users_username_lower_key ON users (LOWER(TRIM(username)))`).Error
	}
	return nil
}

func migrateLOGDB() error {
	var err error
	if err = LOG_DB.AutoMigrate(&Log{}); err != nil {
		return err
	}
	return nil
}

type sqliteColumnDef struct {
	Name string
	DDL  string
}

func ensureSubscriptionPlanTableSQLite() error {
	if !common.UsingSQLite {
		return nil
	}
	tableName := "subscription_plans"
	if !DB.Migrator().HasTable(tableName) {
		createSQL := `CREATE TABLE ` + "`" + tableName + "`" + ` (
` + "`id`" + ` integer,
` + "`title`" + ` varchar(128) NOT NULL,
` + "`subtitle`" + ` varchar(255) DEFAULT '',
` + "`price_amount`" + ` decimal(10,6) NOT NULL,
` + "`currency`" + ` varchar(8) NOT NULL DEFAULT 'USD',
` + "`duration_unit`" + ` varchar(16) NOT NULL DEFAULT 'month',
` + "`duration_value`" + ` integer NOT NULL DEFAULT 1,
` + "`custom_seconds`" + ` bigint NOT NULL DEFAULT 0,
` + "`enabled`" + ` numeric DEFAULT 1,
` + "`sort_order`" + ` integer DEFAULT 0,
` + "`allow_balance_pay`" + ` numeric DEFAULT 1,
` + "`stripe_price_id`" + ` varchar(128) DEFAULT '',
` + "`creem_product_id`" + ` varchar(128) DEFAULT '',
` + "`waffo_pancake_product_id`" + ` varchar(128) DEFAULT '',
` + "`max_purchase_per_user`" + ` integer DEFAULT 0,
` + "`upgrade_group`" + ` varchar(64) DEFAULT '',
` + "`total_amount`" + ` bigint NOT NULL DEFAULT 0,
` + "`quota_reset_period`" + ` varchar(16) DEFAULT 'never',
` + "`quota_reset_custom_seconds`" + ` bigint DEFAULT 0,
` + "`created_at`" + ` bigint,
` + "`updated_at`" + ` bigint,
PRIMARY KEY (` + "`id`" + `)
)`
		return DB.Exec(createSQL).Error
	}
	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := DB.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		existing[c.Name] = struct{}{}
	}
	required := []sqliteColumnDef{
		{Name: "title", DDL: "`title` varchar(128) NOT NULL"},
		{Name: "subtitle", DDL: "`subtitle` varchar(255) DEFAULT ''"},
		{Name: "price_amount", DDL: "`price_amount` decimal(10,6) NOT NULL"},
		{Name: "currency", DDL: "`currency` varchar(8) NOT NULL DEFAULT 'USD'"},
		{Name: "duration_unit", DDL: "`duration_unit` varchar(16) NOT NULL DEFAULT 'month'"},
		{Name: "duration_value", DDL: "`duration_value` integer NOT NULL DEFAULT 1"},
		{Name: "custom_seconds", DDL: "`custom_seconds` bigint NOT NULL DEFAULT 0"},
		{Name: "enabled", DDL: "`enabled` numeric DEFAULT 1"},
		{Name: "sort_order", DDL: "`sort_order` integer DEFAULT 0"},
		{Name: "allow_balance_pay", DDL: "`allow_balance_pay` numeric DEFAULT 1"},
		{Name: "stripe_price_id", DDL: "`stripe_price_id` varchar(128) DEFAULT ''"},
		{Name: "creem_product_id", DDL: "`creem_product_id` varchar(128) DEFAULT ''"},
		{Name: "waffo_pancake_product_id", DDL: "`waffo_pancake_product_id` varchar(128) DEFAULT ''"},
		{Name: "max_purchase_per_user", DDL: "`max_purchase_per_user` integer DEFAULT 0"},
		{Name: "upgrade_group", DDL: "`upgrade_group` varchar(64) DEFAULT ''"},
		{Name: "total_amount", DDL: "`total_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "quota_reset_period", DDL: "`quota_reset_period` varchar(16) DEFAULT 'never'"},
		{Name: "quota_reset_custom_seconds", DDL: "`quota_reset_custom_seconds` bigint DEFAULT 0"},
		{Name: "created_at", DDL: "`created_at` bigint"},
		{Name: "updated_at", DDL: "`updated_at` bigint"},
	}
	for _, col := range required {
		if _, ok := existing[col.Name]; ok {
			continue
		}
		if err := DB.Exec("ALTER TABLE `" + tableName + "` ADD COLUMN " + col.DDL).Error; err != nil {
			return err
		}
	}
	return nil
}

// migrateTokenModelLimitsToText migrates model_limits column from varchar(1024) to text
// This is safe to run multiple times - it checks the column type first
func migrateTokenModelLimitsToText() error {
	// SQLite uses type affinity, so TEXT and VARCHAR are effectively the same — no migration needed
	if common.UsingSQLite {
		return nil
	}

	tableName := "tokens"
	columnName := "model_limits"

	if !DB.Migrator().HasTable(tableName) {
		return nil
	}

	if !DB.Migrator().HasColumn(&Token{}, columnName) {
		return nil
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE text`, tableName, columnName)
	} else if common.UsingMySQL {
		var columnType string
		if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.ToLower(columnType) == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s text", tableName, columnName)
	} else {
		return nil
	}

	if alterSQL != "" {
		if err := DB.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("failed to migrate %s.%s to text: %w", tableName, columnName, err)
		}
		common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to text", tableName, columnName))
	}
	return nil
}

func tableHasColumn(tableName string, columnName string) (bool, error) {
	switch {
	case common.UsingSQLite:
		var cols []sqliteColumnDef
		if err := DB.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
			return false, err
		}
		for _, col := range cols {
			if strings.EqualFold(col.Name, columnName) {
				return true, nil
			}
		}
		return false, nil
	case common.UsingPostgreSQL:
		var count int64
		err := DB.Raw(`SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&count).Error
		return count > 0, err
	case common.UsingMySQL:
		var count int64
		err := DB.Raw(`SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&count).Error
		return count > 0, err
	default:
		return DB.Migrator().HasColumn(tableName, columnName), nil
	}
}

func migrateUserSiteAccessStateColumns() error {
	tableName := "user_site_access_states"
	oldColumn := "has_o_auth_binding"
	newColumn := "has_oauth_binding"
	if !DB.Migrator().HasTable(tableName) {
		return nil
	}
	hasOld, err := tableHasColumn(tableName, oldColumn)
	if err != nil {
		return fmt.Errorf("failed to inspect %s.%s: %w", tableName, oldColumn, err)
	}
	hasNew, err := tableHasColumn(tableName, newColumn)
	if err != nil {
		return fmt.Errorf("failed to inspect %s.%s: %w", tableName, newColumn, err)
	}
	if !hasOld || hasNew {
		return nil
	}
	var errRename error
	switch {
	case common.UsingSQLite:
		errRename = DB.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME COLUMN `%s` TO `%s`", tableName, oldColumn, newColumn)).Error
	case common.UsingPostgreSQL:
		errRename = DB.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME COLUMN %s TO %s`, tableName, oldColumn, newColumn)).Error
	case common.UsingMySQL:
		errRename = DB.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME COLUMN `%s` TO `%s`", tableName, oldColumn, newColumn)).Error
	default:
		errRename = DB.Migrator().RenameColumn(tableName, oldColumn, newColumn)
	}
	if errRename != nil {
		return fmt.Errorf("failed to rename %s.%s to %s: %w", tableName, oldColumn, newColumn, errRename)
	}
	common.SysLog(fmt.Sprintf("Successfully renamed %s.%s to %s", tableName, oldColumn, newColumn))
	return nil
}

// migrateSubscriptionPlanPriceAmount migrates price_amount column from float/double to decimal(10,6)
// This is safe to run multiple times - it checks the column type first
func migrateSubscriptionPlanPriceAmount() {
	// SQLite doesn't support ALTER COLUMN, and its type affinity handles this automatically
	// Skip early to avoid GORM parsing the existing table DDL which may cause issues
	if common.UsingSQLite {
		return
	}

	tableName := "subscription_plans"
	columnName := "price_amount"

	// Check if table exists first
	if !DB.Migrator().HasTable(tableName) {
		return
	}

	// Check if column exists
	if !DB.Migrator().HasColumn(&SubscriptionPlan{}, columnName) {
		return
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		// PostgreSQL: Check if already decimal/numeric
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "numeric" {
			return // Already decimal/numeric
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE decimal(10,6) USING %s::decimal(10,6)`,
			tableName, columnName, columnName)
	} else if common.UsingMySQL {
		// MySQL: Check if already decimal
		var columnType string
		if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.HasPrefix(strings.ToLower(columnType), "decimal") {
			return // Already decimal
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s decimal(10,6) NOT NULL DEFAULT 0",
			tableName, columnName)
	} else {
		return
	}

	if alterSQL != "" {
		if err := DB.Exec(alterSQL).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to migrate %s.%s to decimal: %v", tableName, columnName, err))
		} else {
			common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to decimal(10,6)", tableName, columnName))
		}
	}
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func CloseDB() error {
	if LOG_DB != DB {
		err := closeDB(LOG_DB)
		if err != nil {
			return err
		}
	}
	return closeDB(DB)
}

// checkMySQLChineseSupport ensures the MySQL connection and current schema
// default charset/collation can store Chinese characters. It allows common
// Chinese-capable charsets (utf8mb4, utf8, gbk, big5, gb18030) and panics otherwise.
func checkMySQLChineseSupport(db *gorm.DB) error {
	// 仅检测：当前库默认字符集/排序规则 + 各表的排序规则（隐含字符集）

	// Read current schema defaults
	var schemaCharset, schemaCollation string
	err := db.Raw("SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = DATABASE()").Row().Scan(&schemaCharset, &schemaCollation)
	if err != nil {
		return fmt.Errorf("读取当前库默认字符集/排序规则失败 / Failed to read schema default charset/collation: %v", err)
	}

	toLower := func(s string) string { return strings.ToLower(s) }
	// Allowed charsets that can store Chinese text
	allowedCharsets := map[string]string{
		"utf8mb4": "utf8mb4_",
		"utf8":    "utf8_",
		"gbk":     "gbk_",
		"big5":    "big5_",
		"gb18030": "gb18030_",
	}
	isChineseCapable := func(cs, cl string) bool {
		csLower := toLower(cs)
		clLower := toLower(cl)
		if prefix, ok := allowedCharsets[csLower]; ok {
			if clLower == "" {
				return true
			}
			return strings.HasPrefix(clLower, prefix)
		}
		// 如果仅提供了排序规则，尝试按排序规则前缀判断
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(clLower, prefix) {
				return true
			}
		}
		return false
	}

	// 1) 当前库默认值必须支持中文
	if !isChineseCapable(schemaCharset, schemaCollation) {
		return fmt.Errorf("当前库默认字符集/排序规则不支持中文：schema(%s/%s)。请将库设置为 utf8mb4/utf8/gbk/big5/gb18030 / Schema default charset/collation is not Chinese-capable: schema(%s/%s). Please set to utf8mb4/utf8/gbk/big5/gb18030",
			schemaCharset, schemaCollation, schemaCharset, schemaCollation)
	}

	// 2) 所有物理表的排序规则（隐含字符集）必须支持中文
	type tableInfo struct {
		Name      string
		Collation *string
	}
	var tables []tableInfo
	if err := db.Raw("SELECT TABLE_NAME, TABLE_COLLATION FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'").Scan(&tables).Error; err != nil {
		return fmt.Errorf("读取表排序规则失败 / Failed to read table collations: %v", err)
	}

	var badTables []string
	for _, t := range tables {
		// NULL 或空表示继承库默认设置，已在上面校验库默认，视为通过
		if t.Collation == nil || *t.Collation == "" {
			continue
		}
		cl := *t.Collation
		// 仅凭排序规则判断是否中文可用
		ok := false
		lower := strings.ToLower(cl)
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(lower, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			badTables = append(badTables, fmt.Sprintf("%s(%s)", t.Name, cl))
		}
	}

	if len(badTables) > 0 {
		// 限制输出数量以避免日志过长
		maxShow := 20
		shown := badTables
		if len(shown) > maxShow {
			shown = shown[:maxShow]
		}
		return fmt.Errorf(
			"存在不支持中文的表，请修复其排序规则/字符集。示例（最多展示 %d 项）：%v / Found tables not Chinese-capable. Please fix their collation/charset. Examples (showing up to %d): %v",
			maxShow, shown, maxShow, shown,
		)
	}
	return nil
}

var (
	lastPingTime time.Time
	pingMutex    sync.Mutex
)

func PingDB() error {
	pingMutex.Lock()
	defer pingMutex.Unlock()

	if time.Since(lastPingTime) < time.Second*10 {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Error getting sql.DB from GORM: %v", err)
		return err
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Printf("Error pinging DB: %v", err)
		return err
	}

	lastPingTime = time.Now()
	common.SysLog("Database pinged successfully")
	return nil
}
