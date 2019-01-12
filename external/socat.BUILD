genrule(
    name = "copy_config_h",
    srcs = [
        "@//images/socat:config.h",
    ],
    outs = ["config.h"],
    cmd = "cat $(locations @//images/socat:config.h) > $@",
)

cc_binary(
    name = "socat",
    srcs = glob(["*.c", "*.h"]) + ["config.h"],
    visibility = ["//visibility:public"],
    defines = ["_GNU_SOURCE"],
)

