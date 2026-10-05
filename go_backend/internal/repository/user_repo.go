package repository

import (
	"context"
	"strings"

	"go_backend/internal/model"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*model.SysUser, error) {
	var user model.SysUser
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*model.SysUser, error) {
	var user model.SysUser
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindAll(ctx context.Context) ([]*model.SysUser, error) {
	var users []*model.SysUser
	err := r.db.WithContext(ctx).Order("id asc").Find(&users).Error
	return users, err
}

func (r *UserRepository) Create(ctx context.Context, user *model.SysUser) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) Update(ctx context.Context, user *model.SysUser) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.SysUser{}).Error
}

// SubAccount 相关
func (r *UserRepository) FindSubAccountIDs(ctx context.Context, masterUserID int64) ([]int64, error) {
	var subIDs []int64
	err := r.db.WithContext(ctx).Model(&model.UserSubAccount{}).
		Where("master_user_id = ?", masterUserID).
		Pluck("sub_user_id", &subIDs).Error
	return subIDs, err
}

func (r *UserRepository) FindMasterUserIDs(ctx context.Context, subUserID int64) ([]int64, error) {
	var masterIDs []int64
	err := r.db.WithContext(ctx).Model(&model.UserSubAccount{}).
		Where("sub_user_id = ?", subUserID).
		Pluck("master_user_id", &masterIDs).Error
	return masterIDs, err
}

func (r *UserRepository) ReplaceMasterSubAccounts(ctx context.Context, masterUserID int64, subUserIDs []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("master_user_id = ?", masterUserID).Delete(&model.UserSubAccount{}).Error; err != nil {
			return err
		}
		if len(subUserIDs) == 0 {
			return nil
		}
		subs := make([]model.UserSubAccount, 0, len(subUserIDs))
		for _, sid := range subUserIDs {
			subs = append(subs, model.UserSubAccount{
				MasterUserID: masterUserID,
				SubUserID:    sid,
			})
		}
		return tx.Create(&subs).Error
	})
}

func (r *UserRepository) ExistsSubAccount(ctx context.Context, masterUserID, subUserID int64) bool {
	var count int64
	r.db.WithContext(ctx).Model(&model.UserSubAccount{}).
		Where("master_user_id = ? AND sub_user_id = ?", masterUserID, subUserID).
		Count(&count)
	return count > 0
}

func (r *UserRepository) DeleteSubAccountRelationsForUser(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("sub_user_id = ?", userID).Delete(&model.UserSubAccount{}).Error; err != nil {
			return err
		}
		return tx.Where("master_user_id = ?", userID).Delete(&model.UserSubAccount{}).Error
	})
}

// ViewPermission 相关
func (r *UserRepository) FindViewPermissionTargetIDs(ctx context.Context, userID int64) ([]int64, error) {
	var targetIDs []int64
	err := r.db.WithContext(ctx).Model(&model.UserViewPermission{}).
		Where("user_id = ?", userID).
		Pluck("target_user_id", &targetIDs).Error
	return targetIDs, err
}

func (r *UserRepository) ReplaceViewPermissions(ctx context.Context, userID int64, targetIDs []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserViewPermission{}).Error; err != nil {
			return err
		}
		if len(targetIDs) == 0 {
			return nil
		}
		perms := make([]model.UserViewPermission, 0, len(targetIDs))
		for _, tid := range targetIDs {
			perms = append(perms, model.UserViewPermission{
				UserID:       userID,
				TargetUserID: tid,
			})
		}
		return tx.Create(&perms).Error
	})
}

func (r *UserRepository) ExistsViewPermission(ctx context.Context, userID, targetUserID int64) bool {
	var count int64
	r.db.WithContext(ctx).Model(&model.UserViewPermission{}).
		Where("user_id = ? AND target_user_id = ?", userID, targetUserID).
		Count(&count)
	return count > 0
}

// UserLandingPage 相关
func (r *UserRepository) FindLandingPages(ctx context.Context, platformCode string, userID int64) ([]*model.UserLandingPage, error) {
	var pages []*model.UserLandingPage
	q := r.db.WithContext(ctx)
	if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
		q = q.Where("platform_code = ?", strings.ToLower(platformCode))
	}
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	err := q.Find(&pages).Error
	return pages, err
}

func (r *UserRepository) ReplaceLandingPages(ctx context.Context, platformCode string, userID int64, landingPageIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		delQuery := tx.Where("user_id = ?", userID)
		if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
			delQuery = delQuery.Where("platform_code = ?", strings.ToLower(platformCode))
		}
		if err := delQuery.Delete(&model.UserLandingPage{}).Error; err != nil {
			return err
		}

		if len(landingPageIDs) == 0 {
			return nil
		}
		plat := strings.ToLower(platformCode)
		if plat == "" || plat == "all" {
			plat = "rocnovel"
		}
		pages := make([]model.UserLandingPage, 0, len(landingPageIDs))
		for _, lpid := range landingPageIDs {
			pages = append(pages, model.UserLandingPage{
				PlatformCode:  plat,
				UserID:        userID,
				LandingPageID: lpid,
				Timezone:      "CST",
			})
		}
		return tx.Create(&pages).Error
	})
}

func (r *UserRepository) ReplaceLandingPageConfigs(ctx context.Context, platformCode string, userID int64, pages []*model.UserLandingPage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		delQuery := tx.Where("user_id = ?", userID)
		if platformCode != "" && !strings.EqualFold(platformCode, "ALL") {
			delQuery = delQuery.Where("platform_code = ?", strings.ToLower(platformCode))
		}
		if err := delQuery.Delete(&model.UserLandingPage{}).Error; err != nil {
			return err
		}
		if len(pages) == 0 {
			return nil
		}
		return tx.Create(&pages).Error
	})
}

// FindAdminLandingPageIDs 查询所有管理员/超管已配置的落地页ID（用于普通用户隔离）
func (r *UserRepository) FindAdminLandingPageIDs(ctx context.Context, excludeUserID int64) ([]string, error) {
	var adminIDs []int64
	q := r.db.WithContext(ctx).Model(&model.SysUser{}).
		Where("role IN ('ADMIN', 'SUPER_ADMIN') AND status = 1")
	if excludeUserID > 0 {
		q = q.Where("id != ?", excludeUserID)
	}
	if err := q.Pluck("id", &adminIDs).Error; err != nil {
		return nil, err
	}
	if len(adminIDs) == 0 {
		return nil, nil
	}

	var pids []string
	err := r.db.WithContext(ctx).Model(&model.UserLandingPage{}).
		Where("user_id IN ? AND landing_page_id != '__EMPTY__'", adminIDs).
		Distinct().
		Pluck("landing_page_id", &pids).Error
	return pids, err
}
