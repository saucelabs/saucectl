const $RefParser = require("@apidevtools/json-schema-ref-parser");
const fs = require('fs');
const yargs = require('yargs/yargs');
const {hideBin} = require('yargs/helpers');

const args = yargs(hideBin(process.argv))
    .command('bundle', 'Resolve all references of the given schema and bundle it into a single file.')
    .option('schema', {
        alias: 's',
        type: 'string',
        description: 'The input schema filename.'
    })
    .option('out', {
        alias: 'o',
        type: 'string',
        description: 'The output schema filename.'
    })
    .demandOption(['schema', 'out'])
    .argv;

// Use dereference (not bundle): dereference fully inlines every $ref, so each
// property keeps its own sibling keywords (e.g. a framework's `enum`) standing
// alone. bundle() instead replaces external $refs with internal $refs that point
// to a single shared definition; under JSON Schema draft 2020-12 a $ref and its
// sibling `enum` are intersected, which silently narrowed per-framework enums
// (e.g. Playwright's platformName) down to a stale shared definition's values.
// See INT-574.
$RefParser.dereference(args.s, (err, schema) => {
    if (err) {
        console.error(err);
        process.exit(1);
    } else {
        // `schema` is just a normal JavaScript object that contains your entire JSON Schema,
        // including referenced files, fully inlined into a single object.
        let schemaStr = JSON.stringify(schema, null, 2)

        fs.writeFile(args.o, schemaStr, function (err) {
            if (err) {
                console.log(err);
                process.exit(1)
            }

            console.log(`Successfully bundled schema: ${args.o}`)
        });
    }
});