-- ========================================================
-- LTV 统计系统数据库 DDL & 初始数据 (MySQL 8)
-- ========================================================

-- 1. 多平台接入配置表
CREATE TABLE IF NOT EXISTS `platform_config` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `platform_code` VARCHAR(32) NOT NULL UNIQUE COMMENT '平台标识代码，如 rocnovel, flicknovel',
  `platform_name` VARCHAR(64) NOT NULL COMMENT '平台展示名称，如 中文在线, 番茄司南',
  `auth_type` VARCHAR(32) NOT NULL DEFAULT 'TOKEN_COOKIE' COMMENT '鉴权方式: TOKEN_COOKIE, ED25519_KEY',
  `auth_credentials` TEXT DEFAULT NULL COMMENT '鉴权凭据配置 JSON',
  `sync_cron` VARCHAR(32) DEFAULT '0 5 * * * ?' COMMENT '定时同步 Cron 表达式',
  `launch_start_date` DATE DEFAULT NULL COMMENT '投放起始日期',
  `status` INT NOT NULL DEFAULT 1 COMMENT '状态: 1-启用, 0-停用',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多平台接入配置表';

-- 2. 系统用户表
CREATE TABLE IF NOT EXISTS `sys_user` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `username` VARCHAR(50) NOT NULL UNIQUE COMMENT '用户名',
  `password_hash` VARCHAR(100) NOT NULL COMMENT '加盐哈希密码',
  `role` VARCHAR(20) NOT NULL COMMENT '角色: SUPER_ADMIN, ADMIN, USER',
  `status` INT NOT NULL DEFAULT 1 COMMENT '状态: 1-启用, 0-停用',
  `is_master` INT NOT NULL DEFAULT 0 COMMENT '是否为主账号: 0-普通/子账号, 1-主账号',
  `is_settlement` INT NOT NULL DEFAULT 0 COMMENT '是否为结算账号: 0-否, 1-是',
  `perm_predict_payback` INT NOT NULL DEFAULT 0 COMMENT '权限: 回本预测',
  `perm_roi_predict` INT NOT NULL DEFAULT 0 COMMENT '权限: ROI预测 (D30~D90)',
  `perm_global_distribution` INT NOT NULL DEFAULT 0 COMMENT '权限: 平台大盘汇总',
  `perm_export` INT NOT NULL DEFAULT 0 COMMENT '权限: 数据导出',
  `perm_settlement` INT NOT NULL DEFAULT 0 COMMENT '权限: 月份结算',
  `perm_video_gen` INT NOT NULL DEFAULT 0 COMMENT '权限: AI视频生成',
  `allowed_platforms` VARCHAR(255) NOT NULL DEFAULT 'ALL' COMMENT '允许访问平台: ALL 或逗号分隔平台标识',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统用户表';

-- 3. 主账号与子账号绑定关联表
CREATE TABLE IF NOT EXISTS `user_sub_account` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `master_user_id` BIGINT NOT NULL COMMENT '主账号用户ID',
  `sub_user_id` BIGINT NOT NULL COMMENT '子账号用户ID',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  UNIQUE KEY `uk_master_sub` (`master_user_id`, `sub_user_id`),
  INDEX `idx_master_user` (`master_user_id`),
  INDEX `idx_sub_user` (`sub_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='主账号与子账号绑定关联表';

-- 4. 用户-账户视图只读分配关联表
CREATE TABLE IF NOT EXISTS `user_view_permission` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `user_id` BIGINT NOT NULL COMMENT '授权人/管理员ID',
  `target_user_id` BIGINT NOT NULL COMMENT '被授权查看的目标账号ID',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  UNIQUE KEY `uk_user_target` (`user_id`, `target_user_id`),
  INDEX `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户-账户视图只读分配关联表';

-- 5. 用户-落地页/推广ID关联配置表
CREATE TABLE IF NOT EXISTS `user_landing_page` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `platform_code` VARCHAR(32) NOT NULL DEFAULT 'rocnovel' COMMENT '平台代码',
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `landing_page_id` VARCHAR(64) NOT NULL COMMENT '落地页ID / 推广ID',
  `timezone` VARCHAR(32) NOT NULL DEFAULT 'CST' COMMENT '归属时区: CST(北京时间), UTC(协调世界时), ET(美东时间)',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  UNIQUE KEY `idx_user_landing_page` (`platform_code`, `user_id`, `landing_page_id`),
  INDEX `idx_user_id` (`user_id`),
  INDEX `idx_platform_user` (`platform_code`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户-落地页/推广ID关联配置表';

-- 6. 订单原始明细表
CREATE TABLE IF NOT EXISTS `raw_order` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `platform_code` VARCHAR(32) NOT NULL DEFAULT 'rocnovel' COMMENT '平台标识代码',
  `order_id` VARCHAR(64) NOT NULL COMMENT '原始订单号',
  `member_id` VARCHAR(64) NOT NULL COMMENT '用户/会员ID',
  `landing_page_id` VARCHAR(64) DEFAULT NULL COMMENT '落地页ID / 推广ID',
  `register_time_bj` DATETIME NOT NULL COMMENT '注册时间 (北京时间)',
  `register_time_et` DATETIME NOT NULL COMMENT '注册时间 (美东时间)',
  `register_date_et` DATE NOT NULL COMMENT '注册日期 (美东时间)',
  `register_time_utc` DATETIME DEFAULT NULL COMMENT '注册时间 (UTC时间)',
  `register_date_utc` DATE DEFAULT NULL COMMENT '注册日期 (UTC时间)',
  `pay_time_bj` DATETIME NOT NULL COMMENT '支付时间 (北京时间)',
  `pay_time_et` DATETIME NOT NULL COMMENT '支付时间 (美东时间)',
  `pay_date_et` DATE NOT NULL COMMENT '支付日期 (美东时间)',
  `pay_time_utc` DATETIME DEFAULT NULL COMMENT '支付时间 (UTC时间)',
  `pay_date_utc` DATE DEFAULT NULL COMMENT '支付日期 (UTC时间)',
  `order_amount_cent` INT NOT NULL COMMENT '支付金额 (分)',
  `order_amount_usd` DECIMAL(10,2) NOT NULL COMMENT '支付金额 (美元)',
  `is_subs` INT DEFAULT 0 COMMENT '是否订阅: 1-是, 0-否',
  `renew_type` INT DEFAULT 1 COMMENT '续费类型: 1-首充, 2-自动续费',
  `pay_state` INT DEFAULT 1 COMMENT '支付状态: 1-成功',
  `refund_status` INT DEFAULT 0 COMMENT '退款状态: 0-未退款, 1-已退款',
  `raw_payload` TEXT DEFAULT NULL COMMENT '平台原始报文 JSON',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '同步入库时间',
  UNIQUE KEY `uk_platform_order` (`platform_code`, `order_id`),
  INDEX `idx_reg_date` (`register_date_et`),
  INDEX `idx_reg_date_utc` (`register_date_utc`),
  INDEX `idx_landing_page` (`landing_page_id`),
  INDEX `idx_platform_landing` (`platform_code`, `landing_page_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单原始明细表';

-- 7. 投放消耗与备注配置表
CREATE TABLE IF NOT EXISTS `ltv_launch_config` (
  `platform_code` VARCHAR(32) NOT NULL DEFAULT 'rocnovel' COMMENT '平台代码',
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `launch_date` DATE NOT NULL COMMENT '投放日期 (平台基准时区)',
  `spend` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '投放消耗金额 (USD)',
  `remark` VARCHAR(500) DEFAULT '' COMMENT '运营备注',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`platform_code`, `user_id`, `launch_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='投放消耗与备注配置表';

-- 8. LTV 每日统计汇总表
CREATE TABLE IF NOT EXISTS `ltv_daily_stat` (
  `platform_code` VARCHAR(32) NOT NULL DEFAULT 'rocnovel' COMMENT '平台代码',
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `launch_date` DATE NOT NULL COMMENT '投放日期',
  `spend` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '当日消耗',
  `remark` VARCHAR(500) DEFAULT '' COMMENT '运营备注',
  `total_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '累计充值金额',
  `total_refund` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '累计退款金额',
  `total_profit` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '累计毛利',
  `total_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000 COMMENT '整体ROI',
  `sub_user_count` INT NOT NULL DEFAULT 0 COMMENT '首日订阅用户数',
  `sub_user_cost` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '订阅获客成本',
  `day7_sub_user_count` INT DEFAULT NULL COMMENT '7日续订用户数',
  `day7_sub_user_retention` DECIMAL(10,4) DEFAULT NULL COMMENT '7日续订留存率',
  `day15_sub_user_count` INT DEFAULT NULL COMMENT '15日续订用户数',
  `day15_sub_user_retention` DECIMAL(10,4) DEFAULT NULL COMMENT '15日续订留存率',
  `sub_period_days` INT DEFAULT 1 COMMENT '主导订阅周期 (天)',
  `sub_period_distribution` VARCHAR(500) DEFAULT NULL COMMENT '订阅周期分布快照',
  -- Day 1 ~ Day 60 Recharge & ROI
  `day1_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day1_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day2_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day2_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day3_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day3_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day4_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day4_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day5_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day5_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day6_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day6_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day7_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day7_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day8_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day8_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day9_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day9_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day10_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day10_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day11_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day11_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day12_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day12_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day13_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day13_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day14_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day14_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day15_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day15_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day16_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day16_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day17_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day17_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day18_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day18_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day19_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day19_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day20_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day20_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day21_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day21_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day22_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day22_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day23_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day23_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day24_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day24_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day25_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day25_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day26_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day26_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day27_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day27_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day28_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day28_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day29_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day29_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day30_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day30_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day31_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day31_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day32_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day32_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day33_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day33_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day34_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day34_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day35_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day35_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day36_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day36_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day37_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day37_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day38_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day38_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day39_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day39_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day40_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day40_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day41_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day41_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day42_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day42_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day43_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day43_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day44_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day44_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day45_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day45_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day46_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day46_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day47_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day47_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day48_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day48_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day49_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day49_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day50_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day50_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day51_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day51_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day52_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day52_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day53_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day53_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day54_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day54_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day55_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day55_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day56_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day56_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day57_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day57_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day58_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day58_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day59_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day59_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `day60_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00, `day60_roi` DECIMAL(10,4) NOT NULL DEFAULT 0.0000,
  `predicted_payback_days` INT DEFAULT NULL COMMENT '预测回本周期 (天)',
  `predicted_day30_recharge` DECIMAL(10,2) DEFAULT NULL COMMENT '预测D30充值',
  `predicted_day30_roi` DECIMAL(10,4) DEFAULT NULL COMMENT '预测D30 ROI',
  `predicted_day60_recharge` DECIMAL(10,2) DEFAULT NULL COMMENT '预测D60充值',
  `predicted_day60_roi` DECIMAL(10,4) DEFAULT NULL COMMENT '预测D60 ROI',
  `predicted_day90_recharge` DECIMAL(10,2) DEFAULT NULL COMMENT '预测D90充值',
  `predicted_day90_roi` DECIMAL(10,4) DEFAULT NULL COMMENT '预测D90 ROI',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`platform_code`, `user_id`, `launch_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LTV每日统计汇总表';

-- 9. 每日充值分布统计汇总表 (自然日)
CREATE TABLE IF NOT EXISTS `daily_recharge_distribution` (
  `platform_code` VARCHAR(32) NOT NULL DEFAULT 'rocnovel' COMMENT '平台代码',
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `date` DATE NOT NULL COMMENT '自然日',
  `total_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '总充值',
  `single_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '单充金额',
  `subs_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '订阅金额',
  `total_paid_users` INT NOT NULL DEFAULT 0 COMMENT '总充值人数',
  `single_paid_users` INT NOT NULL DEFAULT 0 COMMENT '单充人数',
  `subs_paid_users` INT NOT NULL DEFAULT 0 COMMENT '订阅人数',
  `new_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '新用户充值',
  `new_recharge_ratio` DECIMAL(10,4) NOT NULL DEFAULT 0.0000 COMMENT '新用户充值占比',
  `new_arpu` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '新用户ARPU',
  `new_paid_users` INT NOT NULL DEFAULT 0 COMMENT '新充值人数',
  `new_single_paid_users` INT NOT NULL DEFAULT 0 COMMENT '新用户单充人数',
  `new_subs_paid_users` INT NOT NULL DEFAULT 0 COMMENT '新用户订阅人数',
  `old_recharge` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '老用户充值',
  `old_recharge_ratio` DECIMAL(10,4) NOT NULL DEFAULT 0.0000 COMMENT '老用户充值占比',
  `old_arpu` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '老用户ARPU',
  `old_paid_users` INT NOT NULL DEFAULT 0 COMMENT '老用户充值人数',
  `old_single_paid_users` INT NOT NULL DEFAULT 0 COMMENT '老用户单充人数',
  `old_subs_paid_users` INT NOT NULL DEFAULT 0 COMMENT '老用户订阅人数',
  `repeat_paid_users` INT NOT NULL DEFAULT 0 COMMENT '整体复充人数',
  `repeat_rate` DECIMAL(10,4) NOT NULL DEFAULT 0.0000 COMMENT '整体复充率',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`platform_code`, `user_id`, `date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='每日充值分布统计汇总表 (自然日)';

-- 10. 月份结算参数配置表
CREATE TABLE IF NOT EXISTS `monthly_settlement_config` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `settlement_type` VARCHAR(32) NOT NULL COMMENT '结算维度: PLATFORM_ALL, USER_ACCOUNT, UNLINKED_PID',
  `target_user_id` BIGINT DEFAULT NULL COMMENT '目标用户ID (USER_ACCOUNT 模式专用)',
  `month_str` VARCHAR(16) NOT NULL COMMENT '结算月份，格式如 2026-07',
  `settled_refund_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '已结算退款金额',
  `month_settled_refund_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '当月结算退款金额',
  `cross_period_refund_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '跨周期退款金额',
  `share_ratio` DECIMAL(6,4) NOT NULL DEFAULT 0.9500 COMMENT '分成比例',
  `channel_fee_rate` DECIMAL(6,4) NOT NULL DEFAULT 0.0700 COMMENT '渠道费率',
  `remark` VARCHAR(500) DEFAULT '' COMMENT '结算备注',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY `uk_settle_type_user_month` (`settlement_type`, `target_user_id`, `month_str`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='月份结算参数配置表';

-- 11. LTV 预测基准数据表 (形态 B)
CREATE TABLE IF NOT EXISTS `ltv_predict_benchmark` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `dimension_type` VARCHAR(32) NOT NULL DEFAULT 'ALL' COMMENT '维度类型: ALL, PLATFORM, etc.',
  `dimension_value` VARCHAR(64) NOT NULL DEFAULT 'DEFAULT' COMMENT '维度值',
  `sub_period_days` INT NOT NULL DEFAULT 1 COMMENT '订阅周期天数',
  `day_index` INT NOT NULL COMMENT '预测天数索引 (1~90)',
  `base_retention_rate` DECIMAL(10,6) NOT NULL DEFAULT 0.000000 COMMENT '基准留存率',
  `base_arpu` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '基准ARPU',
  `sample_cohort_count` INT NOT NULL DEFAULT 0 COMMENT '样本群组数',
  `is_extrapolated` INT NOT NULL DEFAULT 0 COMMENT '是否为外推拟合值',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY `uk_dim_period_day` (`dimension_type`, `dimension_value`, `sub_period_days`, `day_index`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LTV 预测基准数据表';

-- 12. 订阅配置与落地页版本快照表
CREATE TABLE IF NOT EXISTS `subscription_config_version` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `platform_code` VARCHAR(32) NOT NULL DEFAULT 'rocnovel' COMMENT '平台代码',
  `landing_page_id` VARCHAR(64) NOT NULL COMMENT '落地页ID',
  `subscribe_config_id` VARCHAR(64) NOT NULL COMMENT '订阅配置ID',
  `subscribe_config_name` VARCHAR(128) DEFAULT '' COMMENT '订阅配置名称',
  `sale_combo_id` VARCHAR(64) DEFAULT NULL COMMENT '售卖套餐ID',
  `sale_combo_name` VARCHAR(128) DEFAULT '' COMMENT '售卖套餐名称',
  `product_id` VARCHAR(64) NOT NULL COMMENT '商品ID',
  `product_name` VARCHAR(128) DEFAULT '' COMMENT '商品名称',
  `sub_period_days` INT NOT NULL DEFAULT 1 COMMENT '订阅周期天数',
  `first_price_cent` INT NOT NULL COMMENT '首充价格 (分)',
  `renew_price_cent` INT NOT NULL COMMENT '续费价格 (分)',
  `version_num` INT NOT NULL DEFAULT 1 COMMENT '版本号',
  `effective_start_time` DATETIME NOT NULL COMMENT '生效开始时间',
  `effective_end_time` DATETIME DEFAULT NULL COMMENT '生效结束时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  INDEX `idx_pid_first_price` (`landing_page_id`, `first_price_cent`, `effective_start_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订阅配置与落地页版本快照表';

-- 13. 用户-订阅周期与配置关联表
CREATE TABLE IF NOT EXISTS `user_subscription_period` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `platform_code` VARCHAR(32) NOT NULL DEFAULT 'rocnovel' COMMENT '平台代码',
  `member_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
  `landing_page_id` VARCHAR(64) DEFAULT NULL COMMENT '落地页ID',
  `subscribe_config_id` VARCHAR(64) DEFAULT NULL COMMENT '订阅配置ID',
  `sub_period_days` INT NOT NULL DEFAULT 1 COMMENT '订阅周期 (天)',
  `first_price_cent` INT DEFAULT NULL COMMENT '首充金额 (分)',
  `renew_price_cent` INT DEFAULT NULL COMMENT '续费金额 (分)',
  `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '首次订阅时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  INDEX `idx_member_id` (`member_id`),
  INDEX `idx_platform_member` (`platform_code`, `member_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户-订阅周期与配置关联表';

-- 14. 番茄司南推广链接信息表
CREATE TABLE IF NOT EXISTS `flicknovel_promotion` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `promotion_id` VARCHAR(64) NOT NULL COMMENT '番茄司南推广链接ID (promotion_id)',
  `promotion_name` VARCHAR(255) DEFAULT NULL COMMENT '推广名称',
  `recharge_tpl_id` VARCHAR(64) DEFAULT NULL COMMENT '充值模板ID',
  `recharge_tpl_name` VARCHAR(255) DEFAULT NULL COMMENT '充值模板名称',
  `dist_app_id` BIGINT DEFAULT NULL COMMENT '分发应用ID',
  `drama_id` VARCHAR(64) DEFAULT NULL COMMENT '短剧ID',
  `drama_title` VARCHAR(255) DEFAULT NULL COMMENT '短剧标题',
  `chapter_id` VARCHAR(64) DEFAULT NULL COMMENT '章节ID',
  `chapter_title` VARCHAR(255) DEFAULT NULL COMMENT '章节标题',
  `media_channel` INT DEFAULT NULL COMMENT '媒体渠道: 1-FB, 2-TikTok, 3-Google 等',
  `raw_payload` TEXT DEFAULT NULL COMMENT '推广链接原始报文 JSON',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY `uk_fn_prmt_promotion_id` (`promotion_id`),
  INDEX `idx_fn_prmt_recharge_tpl_id` (`recharge_tpl_id`),
  INDEX `idx_fn_prmt_dist_app_id` (`dist_app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='番茄司南推广链接信息表';

-- 15. 番茄司南充值模板表
CREATE TABLE IF NOT EXISTS `flicknovel_recharge_template` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `template_id` VARCHAR(64) NOT NULL COMMENT '充值模板ID',
  `name` VARCHAR(255) DEFAULT NULL COMMENT '模板名称',
  `dist_app_id` BIGINT DEFAULT NULL COMMENT '分发应用ID',
  `price_config_json` TEXT DEFAULT NULL COMMENT '解析后价格字典 JSON，如 {"1199": 0, "1999": 1}',
  `raw_payload` MEDIUMTEXT DEFAULT NULL COMMENT '充值模板完整原始报文',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY `uk_fn_tpl_template_id` (`template_id`),
  INDEX `idx_fn_tpl_dist_app_id` (`dist_app_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='番茄司南充值模板表';

-- 16. 番茄司南染色归因明细表
CREATE TABLE IF NOT EXISTS `flicknovel_relation` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `relation_id` VARCHAR(64) NOT NULL COMMENT '归因关系唯一ID',
  `device_id` VARCHAR(128) NOT NULL COMMENT '设备ID (匿名唯一标识)',
  `promotion_id` VARCHAR(64) DEFAULT NULL COMMENT '关联推广链接ID',
  `promotion_code` VARCHAR(64) DEFAULT NULL COMMENT '推广代码',
  `ad_id` VARCHAR(64) DEFAULT NULL COMMENT '广告ID',
  `adset_id` VARCHAR(64) DEFAULT NULL COMMENT '广告组ID',
  `campaign_id` VARCHAR(64) DEFAULT NULL COMMENT '广告系列ID',
  `ad_account_id` VARCHAR(64) DEFAULT NULL COMMENT '广告账户ID',
  `relation_begin_time_bj` DATETIME DEFAULT NULL COMMENT '归因生效时间 (北京时间)',
  `relation_begin_time_et` DATETIME DEFAULT NULL COMMENT '归因生效时间 (美东时间)',
  `relation_begin_date_et` DATE DEFAULT NULL COMMENT '归因生效日期 (美东时间)',
  `relation_begin_timestamp` BIGINT DEFAULT NULL COMMENT '归因生效毫秒级时间戳',
  `media_channel` VARCHAR(64) DEFAULT NULL COMMENT '投放媒体渠道',
  `platform` VARCHAR(32) DEFAULT NULL COMMENT '客户端系统: ANDROID, IOS',
  `app_id` VARCHAR(32) DEFAULT NULL COMMENT '应用ID',
  `raw_payload` TEXT DEFAULT NULL COMMENT '归因原始明细报文 JSON',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY `uk_fn_relation_id` (`relation_id`),
  INDEX `idx_fn_device_id` (`device_id`),
  INDEX `idx_fn_promotion_id` (`promotion_id`),
  INDEX `idx_fn_begin_time_bj` (`relation_begin_time_bj`),
  INDEX `idx_fn_begin_date_et` (`relation_begin_date_et`),
  INDEX `idx_fn_media_channel` (`media_channel`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='番茄司南染色归因明细表';

-- 17. 系统动态 KV 配置表
CREATE TABLE IF NOT EXISTS `system_config` (
  `config_key` VARCHAR(64) NOT NULL PRIMARY KEY COMMENT '配置项键名',
  `config_value` VARCHAR(2000) DEFAULT NULL COMMENT '配置项取值',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统动态KV配置表';

-- ========================================================
-- 初始化基础数据 (默认管理员与平台配置)
-- ========================================================

-- 初始超级管理员 superadmin (密码: superadmin) 与 管理员 admin (密码: admin666)
INSERT INTO `sys_user` (`username`, `password_hash`, `role`, `status`, `is_master`, `is_settlement`, `perm_predict_payback`, `perm_roi_predict`, `perm_global_distribution`, `perm_export`, `perm_settlement`, `perm_video_gen`, `allowed_platforms`)
VALUES 
('superadmin', 'uOBg1b0FMtNSaOuzsCapBMvyrm9khHEFvmWFTEnNd34=', 'SUPER_ADMIN', 1, 0, 0, 1, 1, 1, 1, 1, 1, 'ALL'),
('admin', '2nhznNAVSq2E77f2pGW167PRovahw+2eCGxAVdbVAng=', 'ADMIN', 1, 0, 0, 1, 1, 1, 1, 1, 1, 'ALL')
ON DUPLICATE KEY UPDATE `status` = VALUES(`status`);

-- 初始支持的多平台接入配置 (中文在线、番茄司南)
INSERT INTO `platform_config` (`platform_code`, `platform_name`, `auth_type`, `launch_start_date`, `status`, `auth_credentials`)
VALUES
('rocnovel', '中文在线', 'TOKEN_COOKIE', '2026-07-10', 1, NULL),
('flicknovel', '番茄司南', 'ED25519_KEY', '2026-09-16', 1, '{\"companyId\":\"355549587538358272\",\"privateKey\":\"ymcPnTqpiQOAtROHJoeegoovJxS7wv6t0HLDUv5q3/G4qry6yKcvjYwhrBqwuEIMjfXMIIqDe0YUPu9JaPofMQ==\"}')
ON DUPLICATE KEY UPDATE `platform_name` = VALUES(`platform_name`), `auth_type` = VALUES(`auth_type`), `launch_start_date` = VALUES(`launch_start_date`);
