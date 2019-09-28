def _impl(ctx):
    out_dir_name = ctx.attr.name
    out_dir = ctx.actions.declare_directory("%s" % out_dir_name)

    all_out_files = []
    commands = []
    all_inputs = []
    for target, f_dest_path in ctx.attr.files.items():
        target_files = target.files.to_list()
        if len(target_files) != 1:
            fail("Each input must describe exactly one file.", attr = "files")
        #out_file = ctx.actions.declare_file("%s/%s" % (out_dir_name, f_dest_path))
        #out_file = ctx.actions.declare_file("%s" % (f_dest_path))
        parent_dir = out_dir_name
        out_path = "%s/%s" % (out_dir.path, f_dest_path)
        cmd = "mkdir -p '%s' && cp '%s' '%s'" % (parent_dir, target_files[0].path, out_path)
        #ctx.actions.run_shell(
        #    outputs = [out_file],
        #    inputs = [target_files[0]], #, out_dir],
        #    command = cmd,
        #)
        commands.append(cmd)
        all_inputs.append(target_files[0])
        #all_out_files.append(out_file)

    ctx.actions.run_shell(
        outputs = [out_dir],
        inputs = all_inputs,
        command = "&&".join(commands),
    )
    all_out_files.append(out_dir)

    return DefaultInfo(
        files = depset(all_out_files),
        #files = depset([out_dir]),
    )

create_dir = rule(
    implementation = _impl,
    attrs = {
        "files": attr.label_keyed_string_dict(mandatory = True, allow_files=True),
    },
)
