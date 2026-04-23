package portal

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
)

type accountHierarchyRow struct {
	ConsumerName       string
	ParentConsumerName string
}

func (s *Service) ResolveAccessibleConsumer(ctx context.Context, operatorConsumerName string, targetConsumerName string) (string, error) {
	operator := model.NormalizeUsername(operatorConsumerName)
	if operator == "" {
		return "", apperr.New(401, "unauthorized")
	}

	target := model.NormalizeUsername(targetConsumerName)
	if target == "" || target == operator {
		return operator, nil
	}

	managed, err := s.listManagedDescendantSet(ctx, operator)
	if err != nil {
		return "", err
	}
	if _, ok := managed[target]; !ok {
		return "", apperr.New(403, "target account is not manageable")
	}
	return target, nil
}

func (s *Service) ResolveManagedDescendant(ctx context.Context, operatorConsumerName string, targetConsumerName string) (string, error) {
	operator := model.NormalizeUsername(operatorConsumerName)
	if operator == "" {
		return "", apperr.New(401, "unauthorized")
	}

	target := model.NormalizeUsername(targetConsumerName)
	if target == "" {
		return "", apperr.New(400, "target account is required")
	}
	if target == operator {
		return "", apperr.New(403, "target account must be a descendant account")
	}

	managed, err := s.listManagedDescendantSet(ctx, operator)
	if err != nil {
		return "", err
	}
	if _, ok := managed[target]; !ok {
		return "", apperr.New(403, "target account is not manageable")
	}
	return target, nil
}

func (s *Service) ListManagedAccounts(ctx context.Context, operatorConsumerName string) ([]model.ManagedAccountSummary, error) {
	operator := model.NormalizeUsername(operatorConsumerName)
	if operator == "" {
		return nil, apperr.New(401, "unauthorized")
	}

	consumers, err := s.listManagedDescendants(ctx, operator)
	if err != nil {
		return nil, err
	}
	return s.listAccountSummaries(ctx, consumers)
}

func (s *Service) ListManagedDepartments(ctx context.Context, operatorConsumerName string) ([]model.ManagedDepartmentNode, error) {
	operator := model.NormalizeUsername(operatorConsumerName)
	if operator == "" {
		return nil, apperr.New(401, "unauthorized")
	}
	orgContext, err := s.loadUserOrgContext(ctx, operator)
	if err != nil {
		return nil, err
	}
	if !orgContext.IsDepartmentAdmin || strings.TrimSpace(orgContext.DepartmentID) == "" {
		return nil, apperr.New(403, "department admin required")
	}
	departmentIDs, err := s.listDepartmentIdsInSubtree(ctx, orgContext.DepartmentID)
	if err != nil {
		return nil, err
	}
	query, args := buildStringInQuery(`
		SELECT
			d.department_id,
			d.name,
			COALESCE(d.parent_department_id, '') AS parent_department_id,
			COALESCE(d.admin_consumer_name, '') AS admin_consumer_name,
			COALESCE(d.path, '') AS path,
			COALESCE(members.cnt, 0) AS member_count
		FROM org_department d
		LEFT JOIN (
			SELECT m.department_id, COUNT(1) AS cnt
			FROM org_account_membership m
			JOIN portal_user u ON u.consumer_name = m.consumer_name
			WHERE department_id IS NOT NULL
			  AND COALESCE(u.is_deleted, FALSE) = FALSE
			GROUP BY department_id
		) members ON members.department_id = d.department_id
		WHERE d.department_id IN (%s)
		ORDER BY d.level ASC, d.sort_order ASC, d.name ASC`, departmentIDs)
	records, err := s.db.GetAll(ctx, query, args...)
	if err != nil {
		return nil, gerror.Wrap(err, "query managed departments failed")
	}

	nodeMap := make(map[string]model.ManagedDepartmentNode, len(records))
	childrenByParent := make(map[string][]string, len(records))
	for _, record := range records {
		deptID := strings.TrimSpace(record["department_id"].String())
		if deptID == "" || deptID == orgRootDepartmentID {
			continue
		}
		_, departmentPath, pathErr := s.resolveDepartmentPath(ctx, deptID)
		if pathErr != nil {
			return nil, pathErr
		}
		nodeMap[deptID] = model.ManagedDepartmentNode{
			DepartmentID:       deptID,
			Name:               strings.TrimSpace(record["name"].String()),
			DepartmentPath:     departmentPath,
			ParentDepartmentID: strings.TrimSpace(record["parent_department_id"].String()),
			AdminConsumerName:  strings.TrimSpace(record["admin_consumer_name"].String()),
			MemberCount:        record["member_count"].Int64(),
			Children:           []model.ManagedDepartmentNode{},
		}
	}
	rootIDs := make([]string, 0)
	for _, node := range nodeMap {
		parentID := strings.TrimSpace(node.ParentDepartmentID)
		if parentID == "" || parentID == orgRootDepartmentID || node.DepartmentID == orgContext.DepartmentID {
			rootIDs = append(rootIDs, node.DepartmentID)
			continue
		}
		if _, ok := nodeMap[parentID]; !ok {
			rootIDs = append(rootIDs, node.DepartmentID)
			continue
		}
		childrenByParent[parentID] = append(childrenByParent[parentID], node.DepartmentID)
	}
	if len(rootIDs) == 0 {
		if _, ok := nodeMap[orgContext.DepartmentID]; ok {
			rootIDs = append(rootIDs, orgContext.DepartmentID)
		}
	}
	roots := make([]model.ManagedDepartmentNode, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		if _, ok := nodeMap[rootID]; ok {
			roots = append(roots, buildManagedDepartmentTree(rootID, nodeMap, childrenByParent))
		}
	}
	return roots, nil
}

func buildManagedDepartmentTree(rootID string, nodeMap map[string]model.ManagedDepartmentNode,
	childrenByParent map[string][]string,
) model.ManagedDepartmentNode {
	node := nodeMap[rootID]
	childIDs := childrenByParent[rootID]
	node.Children = make([]model.ManagedDepartmentNode, 0, len(childIDs))
	for _, childID := range childIDs {
		node.Children = append(node.Children, buildManagedDepartmentTree(childID, nodeMap, childrenByParent))
	}
	return node
}

func (s *Service) UpdateManagedAccount(ctx context.Context, operatorConsumerName string, targetConsumerName string,
	req model.UpdateManagedAccountRequest,
) (model.ManagedAccountSummary, error) {
	target, err := s.ResolveManagedDescendant(ctx, operatorConsumerName, targetConsumerName)
	if err != nil {
		return model.ManagedAccountSummary{}, err
	}

	user, err := s.getUserByName(ctx, target)
	if err != nil {
		return model.ManagedAccountSummary{}, err
	}
	if user == nil {
		return model.ManagedAccountSummary{}, apperr.New(404, "target account not found")
	}

	userLevel := normalizeUserLevel(req.UserLevel)
	status, err := normalizeManagedAccountStatus(req.Status)
	if err != nil {
		return model.ManagedAccountSummary{}, err
	}

	if _, err = s.db.Exec(ctx, `
		UPDATE portal_user
		SET user_level = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE consumer_name = ?`,
		userLevel,
		status,
		target,
	); err != nil {
		return model.ManagedAccountSummary{}, gerror.Wrap(err, "update managed account failed")
	}
	if err = s.syncKeyAuthConsumers(ctx); err != nil {
		return model.ManagedAccountSummary{}, apperr.New(503,
			"managed account updated but failed to sync gateway key-auth", err.Error())
	}

	summary, err := s.getSingleManagedAccountSummary(ctx, target)
	if err != nil {
		return model.ManagedAccountSummary{}, err
	}
	if summary == nil {
		return model.ManagedAccountSummary{}, apperr.New(404, "target account not found")
	}
	return *summary, nil
}

func (s *Service) AdjustManagedAccountBalance(ctx context.Context, operatorConsumerName string, targetConsumerName string,
	req model.AdjustManagedAccountBalanceRequest,
) (model.ManagedAccountSummary, error) {
	target, err := s.ResolveManagedDescendant(ctx, operatorConsumerName, targetConsumerName)
	if err != nil {
		return model.ManagedAccountSummary{}, err
	}

	deltaMicroYuan := rmbToMicroYuan(req.Amount)
	if deltaMicroYuan == 0 {
		return model.ManagedAccountSummary{}, apperr.New(400, "amount must not be 0")
	}

	normalizedOperator := model.NormalizeUsername(operatorConsumerName)
	now := model.NowInAppLocation()
	adjustmentID := fmt.Sprintf("BA%d%s", time.Now().UnixMilli(), strings.ToUpper(randomString(4)))
	reason := strings.TrimSpace(req.Reason)
	txID := "a" + sha256Hex("portal_balance_adjustment:" + adjustmentID)[:32]

	err = s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec(`
			INSERT INTO portal_balance_adjustment
			(adjustment_id, operator_consumer_name, target_consumer_name, delta_micro_yuan, reason, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			adjustmentID,
			normalizedOperator,
			target,
			deltaMicroYuan,
			reason,
			now,
		); txErr != nil {
			return gerror.Wrap(txErr, "insert balance adjustment failed")
		}

		if _, txErr := tx.Exec(`
			INSERT INTO billing_transaction
			(tx_id, consumer_name, tx_type, amount_micro_yuan, currency, source_type, source_id, occurred_at, created_at)
			VALUES (?, ?, 'adjust', ?, 'CNY', 'portal_balance_adjustment', ?, ?, ?)`,
			txID,
			target,
			deltaMicroYuan,
			adjustmentID,
			now,
			now,
		); txErr != nil {
			return gerror.Wrap(txErr, "insert balance transaction failed")
		}

		if _, txErr := tx.Exec(`
			INSERT INTO billing_wallet
			(consumer_name, currency, available_micro_yuan, version)
			VALUES (?, 'CNY', ?, 1)
			`+s.upsertClause([]string{"consumer_name"},
			s.upsertAdd("billing_wallet", "available_micro_yuan"),
			s.upsertAdd("billing_wallet", "version"))+``,
			target,
			deltaMicroYuan,
		); txErr != nil {
			return gerror.Wrap(txErr, "update billing wallet failed")
		}
		return nil
	})
	if err != nil {
		return model.ManagedAccountSummary{}, gerror.Wrap(err, "adjust managed account balance failed")
	}
	if err = s.syncConsumerBalanceToRedis(ctx, target); err != nil {
		s.logf(ctx, "sync managed balance to redis failed: consumer=%s err=%v", target, err)
	}

	summary, err := s.getSingleManagedAccountSummary(ctx, target)
	if err != nil {
		return model.ManagedAccountSummary{}, err
	}
	if summary == nil {
		return model.ManagedAccountSummary{}, apperr.New(404, "target account not found")
	}
	return *summary, nil
}

func (s *Service) getSingleManagedAccountSummary(ctx context.Context, consumerName string) (*model.ManagedAccountSummary, error) {
	items, err := s.listAccountSummaries(ctx, []string{consumerName})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (s *Service) listAccountSummaries(ctx context.Context, consumerNames []string) ([]model.ManagedAccountSummary, error) {
	if len(consumerNames) == 0 {
		return []model.ManagedAccountSummary{}, nil
	}

	query, args := buildConsumerInQuery(`
		SELECT u.consumer_name, u.display_name, u.email, u.user_level, u.status,
			COALESCE(m.department_id, '') AS department_id,
			COALESCE(m.parent_consumer_name, '') AS parent_consumer_name
		FROM portal_user u
		LEFT JOIN org_account_membership m ON m.consumer_name = u.consumer_name
		WHERE u.consumer_name IN (%s)
		  AND COALESCE(u.is_deleted, FALSE) = FALSE
		ORDER BY u.consumer_name ASC`, consumerNames)
	records, err := s.db.GetAll(ctx, query, args...)
	if err != nil {
		return nil, gerror.Wrap(err, "query managed accounts failed")
	}
	if len(records) == 0 {
		return []model.ManagedAccountSummary{}, nil
	}

	balanceMap, err := s.queryConsumerMetricMap(ctx, `
		SELECT consumer_name, available_micro_yuan AS metric_value
		FROM billing_wallet
		WHERE consumer_name IN (%s)`, consumerNames)
	if err != nil {
		return nil, err
	}
	consumptionMap, err := s.queryConsumerMetricMap(ctx, `
		SELECT consumer_name, COALESCE(SUM(0 - amount_micro_yuan), 0) AS metric_value
		FROM billing_transaction
		WHERE consumer_name IN (%s)
		  AND tx_type IN ('consume', 'reconcile')
		  AND amount_micro_yuan < 0
		GROUP BY consumer_name`, consumerNames)
	if err != nil {
		return nil, err
	}
	activeKeyMap, err := s.queryConsumerMetricMap(ctx, `
		SELECT consumer_name, COUNT(1) AS metric_value
		FROM portal_api_key
		WHERE consumer_name IN (%s)
		  AND deleted_at IS NULL
		  AND status = 'active'
		  AND (expires_at IS NULL OR expires_at > ?)
		GROUP BY consumer_name`, consumerNames, model.NowInAppLocation())
	if err != nil {
		return nil, err
	}

	items := make([]model.ManagedAccountSummary, 0, len(records))
	for _, record := range records {
		consumerName := model.NormalizeUsername(record["consumer_name"].String())
		if consumerName == "" {
			continue
		}

		orgContext, ctxErr := s.loadUserOrgContext(ctx, consumerName)
		if ctxErr != nil {
			return nil, ctxErr
		}
		items = append(items, model.ManagedAccountSummary{
			ConsumerName:       consumerName,
			DisplayName:        strings.TrimSpace(record["display_name"].String()),
			Email:              strings.TrimSpace(record["email"].String()),
			DepartmentID:       orgContext.DepartmentID,
			DepartmentName:     orgContext.DepartmentName,
			DepartmentPath:     orgContext.DepartmentPath,
			ParentConsumerName: strings.TrimSpace(record["parent_consumer_name"].String()),
			AdminConsumerName:  orgContext.AdminConsumerName,
			IsDepartmentAdmin:  orgContext.IsDepartmentAdmin,
			UserLevel:          normalizeUserLevel(record["user_level"].String()),
			Status:             normalizeManagedAccountStatusOrDefault(record["status"].String()),
			Balance:            microYuanToText(balanceMap[consumerName]),
			TotalConsumption:   microYuanToText(consumptionMap[consumerName]),
			ActiveKeys:         activeKeyMap[consumerName],
		})
	}
	return items, nil
}

func (s *Service) queryConsumerMetricMap(ctx context.Context, queryTemplate string, consumerNames []string,
	extraArgs ...any,
) (map[string]int64, error) {
	query, args := buildConsumerInQuery(queryTemplate, consumerNames)
	args = append(args, extraArgs...)
	records, err := s.db.GetAll(ctx, query, args...)
	if err != nil {
		return nil, gerror.Wrap(err, "query managed account metrics failed")
	}

	result := make(map[string]int64, len(records))
	for _, record := range records {
		consumerName := model.NormalizeUsername(record["consumer_name"].String())
		if consumerName == "" {
			continue
		}
		result[consumerName] = record["metric_value"].Int64()
	}
	return result, nil
}

func (s *Service) listManagedDescendants(ctx context.Context, operatorConsumerName string) ([]string, error) {
	operator := model.NormalizeUsername(operatorConsumerName)
	if operator == "" {
		return nil, apperr.New(401, "unauthorized")
	}

	orgContext, err := s.loadUserOrgContext(ctx, operator)
	if err != nil {
		return nil, err
	}
	if !orgContext.IsDepartmentAdmin || strings.TrimSpace(orgContext.DepartmentID) == "" {
		return nil, apperr.New(403, "department admin required")
	}

	departmentIDs, err := s.listDepartmentIdsInSubtree(ctx, orgContext.DepartmentID)
	if err != nil {
		return nil, err
	}
	query, args := buildStringInQuery(`
		SELECT consumer_name
		FROM org_account_membership
		WHERE department_id IN (%s)
		  AND consumer_name <> ?
		ORDER BY consumer_name ASC`, departmentIDs)
	args = append(args, operator)
	records, err := s.db.GetAll(ctx, query, args...)
	if err != nil {
		return nil, gerror.Wrap(err, "query managed department accounts failed")
	}

	items := make([]string, 0, len(records))
	for _, record := range records {
		consumerName := model.NormalizeUsername(record["consumer_name"].String())
		if consumerName != "" {
			items = append(items, consumerName)
		}
	}
	return items, nil
}

func (s *Service) listManagedDescendantSet(ctx context.Context, operatorConsumerName string) (map[string]struct{}, error) {
	consumers, err := s.listManagedDescendants(ctx, operatorConsumerName)
	if err != nil {
		return nil, err
	}
	items := make(map[string]struct{}, len(consumers))
	for _, consumerName := range consumers {
		items[consumerName] = struct{}{}
	}
	return items, nil
}

func collectManagedDescendants(rows []accountHierarchyRow, operatorConsumerName string) []string {
	operator := model.NormalizeUsername(operatorConsumerName)
	if operator == "" {
		return []string{}
	}

	childrenByParent := make(map[string][]string, len(rows))
	for _, row := range rows {
		consumerName := model.NormalizeUsername(row.ConsumerName)
		parentConsumerName := model.NormalizeUsername(row.ParentConsumerName)
		if consumerName == "" || parentConsumerName == "" || consumerName == operator {
			continue
		}
		childrenByParent[parentConsumerName] = append(childrenByParent[parentConsumerName], consumerName)
	}
	for parent, children := range childrenByParent {
		sort.Strings(children)
		childrenByParent[parent] = children
	}

	queue := []string{operator}
	visited := map[string]struct{}{operator: {}}
	descendants := make([]string, 0)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, child := range childrenByParent[current] {
			if _, ok := visited[child]; ok {
				continue
			}
			visited[child] = struct{}{}
			descendants = append(descendants, child)
			queue = append(queue, child)
		}
	}
	return descendants
}

func buildConsumerInQuery(queryTemplate string, consumerNames []string) (string, []any) {
	placeholders := make([]string, 0, len(consumerNames))
	args := make([]any, 0, len(consumerNames))
	for _, consumerName := range consumerNames {
		normalized := model.NormalizeUsername(consumerName)
		if normalized == "" {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, normalized)
	}
	if len(placeholders) == 0 {
		placeholders = append(placeholders, "''")
	}
	return fmt.Sprintf(queryTemplate, strings.Join(placeholders, ",")), args
}

func normalizeManagedAccountStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case consts.UserStatusActive, consts.UserStatusDisabled, consts.UserStatusPending:
		return strings.ToLower(strings.TrimSpace(status)), nil
	default:
		return "", apperr.New(400, "status must be active, disabled or pending")
	}
}

func normalizeManagedAccountStatusOrDefault(status string) string {
	normalized, err := normalizeManagedAccountStatus(status)
	if err != nil {
		return consts.UserStatusActive
	}
	return normalized
}
