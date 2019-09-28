def _impl(ctx):
    in_file = ctx.file.src

    out_sha1 = ctx.actions.declare_file("%s.sha1" % in_file.path)
    ctx.actions.run(
        executable = ctx.executable._cmd_sha1,
        outputs = [out_sha1],
        inputs = [in_file],
        arguments = [in_file.path, out_sha1.path],
    )

    out_sha256 = ctx.actions.declare_file("%s.sha256" % in_file.path)
    ctx.actions.run(
        executable = ctx.executable._cmd_sha256,
        #tools = ctx.attr._cmd_sha256.files,
        outputs = [out_sha256],
        inputs = [in_file],
        arguments = [in_file.path, out_sha256.path],
    )

    return DefaultInfo(
        files = depset([out_sha1, out_sha256]),
    )

hashes = rule(
    implementation = _impl,
    attrs = {
        "src": attr.label(mandatory = True, allow_single_file = True),
        "_cmd_sha1": attr.label(
            default = Label("//hack/bazel:sha1"),
            allow_single_file = True,
            executable = True,
            cfg = "host",
        ),
        "_cmd_sha256": attr.label(
            default = Label("//hack/bazel:sha256"),
            allow_single_file = True,
            executable = True,
            cfg = "host",
        ),
    },
)
