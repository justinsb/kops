def _impl(ctx):
    # Create actions to generate the three output files.
    # Actions are run only when the corresponding file is requested.

    ctx.actions.run(
        executable = ctx.executable._cmd_sha1,
        tools = ctx.attr._cmd_sha1.files,
        outputs = [ctx.outputs.sha1],
        inputs = [ctx.file.src],
        arguments = [ctx.file.src.path, ctx.outputs.sha1.path],
    )

    ctx.actions.run(
        executable = ctx.executable._cmd_sha256,
        tools = ctx.attr._cmd_sha256.files,
        outputs = [ctx.outputs.sha256],
        inputs = [ctx.file.src],
        arguments = [ctx.file.src.path, ctx.outputs.sha256.path],
    )

    # By default (if you run `bazel build` on this target, or if you use it as a
    # source of another target), only the sha256 is computed.
    return DefaultInfo(files = depset([ctx.outputs.sha256]))

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
    outputs = {
        "sha1": "%{name}.sha1",
        "sha256": "%{name}.sha256",
    },
)
