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
        type: 'array',
        description: 'The output schema filename.'
    })
    .demandOption(['schema', 'out'])
    .argv;

$RefParser.bundle(args.s, (err, schema) => {
    if (err) {
        console.error(err);
        process.exit(1);
    } else {
        // `schema` is just a normal JavaScript object that contains your entire JSON Schema,
        // including referenced files, combined into a single object.
        let schemaStr = JSON.stringify(schema, null, 2)

        for (const outFile of args.o) {
            fs.writeFile(outFile, schemaStr, function (err) {
                if (err) {
                    console.log(err);
                    process.exit(1)
                }

                console.log(`Successfully bundled schema: ${outFile}`)
            });
        }
    }
});