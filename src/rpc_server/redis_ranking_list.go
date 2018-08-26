package main

import (
	"encoding/json"
	"ih_server/libs/log"
	"ih_server/libs/utils"
	"strconv"

	"github.com/gomodule/redigo/redis"
)

const (
	RANKING_LIST_STAGE_TOTAL_SCORE  = "mm:ranking_list_stage_total_score"
	RANKING_LIST_STAGE_SCORE        = "mm:ranking_list_stage_score"
	RANKING_LIST_CHARM              = "mm:ranking_list_charm"
	RANKING_LIST_CAT_OUQI           = "mm:ranking_list_cat_ouqi"
	RANKING_LIST_ZANED              = "mm:ranking_list_zaned"
	STAGE_SCORE_RANKING_LIST_ID_SET = "mm:stage_score_ranking_list_id_set"
)

func (this *RedisGlobalData) LoadStageTotalScoreRankItems() int32 {
	int_map, err := redis.StringMap(global_data.redis_conn.Do("HGETALL", RANKING_LIST_STAGE_TOTAL_SCORE))
	if err != nil {
		log.Error("redis获取集合[%v]数据失败[%v]", RANKING_LIST_STAGE_TOTAL_SCORE, err.Error())
		return -1
	}

	for k, item := range int_map {
		jitem := &RankStageTotalScoreItem{}
		if err := json.Unmarshal([]byte(item), jitem); err != nil {
			log.Error("##### Load StageTotalScoreRankItem item[%v] error[%v]", item, err.Error())
			return -1
		}
		if !ranking_list_proc.stage_total_score_ranking_list.Update(jitem) {
			log.Warn("载入集合[%v]数据[%v,%v]失败", RANKING_LIST_STAGE_TOTAL_SCORE, k, item)
		}
	}
	return 1
}

func (this *RedisGlobalData) LoadStageScoreRankItems() int32 {
	ints, err := redis.Ints(global_data.redis_conn.Do("SMEMBERS", STAGE_SCORE_RANKING_LIST_ID_SET))
	if err != nil {
		log.Error("redis获取集合[%v]数据失败[%v]", STAGE_SCORE_RANKING_LIST_ID_SET, err.Error())
		return -1
	}

	for i := 0; i < len(ints); i++ {
		key := RANKING_LIST_STAGE_SCORE + ":" + strconv.Itoa(ints[i])
		int_map, err := redis.StringMap(global_data.redis_conn.Do("HGETALL", key))
		if err != nil {
			log.Error("redis获取集合[%v]数据失败[%v]", key, err.Error())
			return -1
		}

		for k, item := range int_map {
			jitem := &RankStageScoreItem{}
			if err := json.Unmarshal([]byte(item), jitem); err != nil {
				log.Error("##### Load StageScoreRankItem[%v] item[%v] error[%v]", key, item, err.Error())
				return -1
			}
			rank_list := ranking_list_proc.GetStageScoreRankList(int32(ints[i]))
			if !rank_list.Update(jitem) {
				log.Warn("载入集合[%v]数据[%v,%v]失败", key, k, item)
			}
		}
	}
	return 1
}

func (this *RedisGlobalData) LoadCharmRankItems() int32 {
	int_map, err := redis.StringMap(global_data.redis_conn.Do("HGETALL", RANKING_LIST_CHARM))
	if err != nil {
		log.Error("redis获取集合[%v]数据失败[%v]", RANKING_LIST_CHARM, err.Error())
		return -1
	}

	for k, item := range int_map {
		jitem := &RankCharmItem{}
		if err := json.Unmarshal([]byte(item), jitem); err != nil {
			log.Error("##### Load StageCharmItem key[%v] item[%v] error[%v]", k, item, err.Error())
			return -1
		}
		if !ranking_list_proc.charm_ranking_list.Update(jitem) {
			log.Warn("载入集合[%v]数据[%v,%v]失败", RANKING_LIST_CHARM, k, item)
		}
	}
	return 1
}

func (this *RedisGlobalData) LoadZanedRankItems() int32 {
	int_map, err := redis.StringMap(global_data.redis_conn.Do("HGETALL", RANKING_LIST_ZANED))
	if err != nil {
		log.Error("redis获取集合[%v]数据失败[%v]", RANKING_LIST_ZANED, err.Error())
		return -1
	}

	for k, item := range int_map {
		jitem := &RankZanedItem{}
		if err := json.Unmarshal([]byte(item), jitem); err != nil {
			log.Error("##### Load StageZanedItem key[%v] item[%v] error[%v]", k, item, err.Error())
			return -1
		}
		if !ranking_list_proc.zaned_ranking_list.Update(jitem) {
			log.Warn("载入集合[%v]数据[%v,%v]失败", RANKING_LIST_ZANED, k, item)
		}
	}
	return 1
}

func (this *RedisGlobalData) UpdateRankStageTotalScore(item *RankStageTotalScoreItem) int32 {
	bytes, err := json.Marshal(item)
	if err != nil {
		log.Error("##### Serialize item[%v] error[%v]", *item, err.Error())
		return -1
	}
	err = this.redis_conn.Post("HSET", RANKING_LIST_STAGE_TOTAL_SCORE, item.PlayerId, string(bytes))
	if err != nil {
		log.Error("redis设置集合[%v]数据失败[%v]", RANKING_LIST_STAGE_TOTAL_SCORE, err.Error())
		return -1
	}
	return 1
}

func (this *RedisGlobalData) UpdateRankStageScore(item *RankStageScoreItem) int32 {
	bytes, err := json.Marshal(item)
	if err != nil {
		log.Error("##### Serialize item[%v] error[%v]", *item, err.Error())
		return -1
	}
	err = this.redis_conn.Post("SADD", STAGE_SCORE_RANKING_LIST_ID_SET, item.StageId)
	if err != nil {
		log.Error("redis设置集合[%v]数据失败[%v]", STAGE_SCORE_RANKING_LIST_ID_SET, err.Error())
		return -1
	}
	key := RANKING_LIST_STAGE_SCORE + ":" + strconv.Itoa(int(item.StageId))
	err = this.redis_conn.Post("HSET", key, item.PlayerId, string(bytes))
	if err != nil {
		log.Error("redis设置集合[%v]数据失败[%v]", key, err.Error())
		return -1
	}
	return 1
}

func (this *RedisGlobalData) UpdateRankCharm(item *RankCharmItem) int32 {
	bytes, err := json.Marshal(item)
	if err != nil {
		log.Error("##### Serialize item[%v] error[%v]", *item, err.Error())
		return -1
	}
	err = this.redis_conn.Post("HSET", RANKING_LIST_CHARM, item.PlayerId, string(bytes))
	if err != nil {
		log.Error("redis设置集合[%v]数据失败[%v]", RANKING_LIST_CHARM, err.Error())
		return -1
	}
	return 1
}

func (this *RedisGlobalData) UpdateRankCatOuqi(item *RankOuqiItem) int32 {
	bytes, err := json.Marshal(item)
	if err != nil {
		log.Error("##### Serialize item[%v] error[%v]", *item, err.Error())
		return -1
	}
	err = this.redis_conn.Post("HSET", RANKING_LIST_CAT_OUQI, utils.Int64From2Int32(item.PlayerId, item.CatId), string(bytes))
	if err != nil {
		log.Error("redis设置集合[%v]数据失败[%v]", RANKING_LIST_CAT_OUQI, err.Error())
		return -1
	}
	return 1
}

func (this *RedisGlobalData) UpdateRankZaned(item *RankZanedItem) int32 {
	bytes, err := json.Marshal(item)
	if err != nil {
		log.Error("##### Serialize item[%v] error[%v]", *item, err.Error())
		return -1
	}
	err = this.redis_conn.Post("HSET", RANKING_LIST_ZANED, item.PlayerId, string(bytes))
	if err != nil {
		log.Error("redis设置集合[%v]数据失败[%v]", RANKING_LIST_ZANED, err.Error())
		return -1
	}
	return 1
}

func (this *RedisGlobalData) DeleteRankStageTotalScore(player_id int32) int32 {
	err := this.redis_conn.Post("HDEL", RANKING_LIST_STAGE_TOTAL_SCORE, player_id)
	if err != nil {
		log.Error("redis删除集合[%v]数据失败[%v]", RANKING_LIST_STAGE_TOTAL_SCORE, err.Error())
		return -1
	}
	return 1
}

func (this *RedisGlobalData) DeleteRankStageScore(player_id int32, stage_id int32) int32 {
	err := this.redis_conn.Post("HDEL", RANKING_LIST_STAGE_SCORE, player_id)
	if err != nil {
		log.Error("redis删除集合[%v]数据失败[%v]", RANKING_LIST_STAGE_SCORE, err.Error())
		return -1
	}
	return 1
}

func (this *RedisGlobalData) DeleteRankCharm(player_id int32) int32 {
	err := this.redis_conn.Post("HDEL", RANKING_LIST_CHARM, player_id)
	if err != nil {
		log.Error("redis删除集合[%v]数据失败[%v]", RANKING_LIST_CHARM, err.Error())
		return -1
	}
	return 1
}

func (this *RedisGlobalData) DeleteRankCatOuqi(player_id int32, cat_id int32) int32 {
	err := this.redis_conn.Post("HDEL", RANKING_LIST_CAT_OUQI, utils.Int64From2Int32(player_id, cat_id))
	if err != nil {
		log.Error("redis删除集合[%v]数据失败[%v]", RANKING_LIST_CAT_OUQI, err.Error())
		return -1
	}
	return 1
}

func (this *RedisGlobalData) DeleteRankZaned(player_id int32) int32 {
	err := this.redis_conn.Post("HDEL", RANKING_LIST_ZANED, player_id)
	if err != nil {
		log.Error("redis删除集合[%v]数据失败[%v]", RANKING_LIST_ZANED, err.Error())
		return -1
	}
	return 1
}
