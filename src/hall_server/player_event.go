package main

const (
	EVENT_GLOBAL                     = iota // 全局
	EVENT_ENTER_BATTLE               = 1    // 战斗进场
	EVENT_AFTER_USE_SUPER_SKILL      = 2    // 释放大招后
	EVENT_AFTER_USE_NORMAL_SKILL     = 3    // 释放普攻后
	EVENT_BEFORE_DAMAGE_ON_ATTACK    = 4    // 攻击计算伤害前
	EVENT_BEFORE_DAMAGE_ON_BE_ATTACK = 5    // 被击计算伤害前
	EVENT_AFTER_DAMAGE_ON_ATTACK     = 6    // 任何攻击后
	EVENT_AFTER_DAMAGE_ON_BE_ATTACK  = 7    // 任何被攻击后
	EVENT_BE_BLOCK                   = 8    // 被格挡
	EVENT_BE_CRITICAL                = 9    // 被暴击
	EVENT_CRITICAL                   = 10   // 暴击
	EVENT_BLOCK                      = 11   // 格挡
	EVENT_BEFORE_ROUND               = 12   // 回合结束前
	EVENT_AFTER_TARGET_DEAD          = 13   // 目标死亡后
	EVENT_AFTER_TEAMMATE_DEAD        = 14   // 同伴死亡后
	EVENT_AFTER_ENEMY_DEAD           = 15   // 敌人死亡后
	EVENT_ON_CURE                    = 16   // 治疗时
	EVENT_BEFORE_NORMAL_ATTACK       = 17   // 普攻前
	EVENT_BEFORE_RAGE_ATTACK         = 18   // 怒攻前
	EVENT_KILL_ENEMY                 = 19   // 杀死敌人
	EVENT_BEFORE_TARGET_DEAD         = 20   // 目标死亡前
	EVENT_HP_CHANGED                 = 21   // 血量变化
)
