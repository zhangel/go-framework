package plugin

import "github.com/zhangel/go-framework.git/config/internal"

// 这玩意儿存在的唯一价值是为了解除与Plugin的循环引用……

type Config internal.Config
