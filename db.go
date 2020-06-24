package baas

import (
	"github.com/rz1226/mysqlx"
)

//初始化由使用者执行
var Dbkit *mysqlx.DB

/**

CREATE TABLE `baas_item` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `key` varchar(50) DEFAULT NULL,
  `content` MEDIUMBLOB DEFAULT NULL comment'数据内容',

  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

  `last_update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
  PRIMARY KEY (`id`),
  unique (`key`)

) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4;



*/
