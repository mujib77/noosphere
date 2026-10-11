import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const currentDirectory = dirname(fileURLToPath(import.meta.url));

const schemaPath = resolve(
  currentDirectory,
  "../planetary-event.schema.json"
);

const fixturePath = resolve(
  currentDirectory,
  "../examples/weather-observation.json"
);

const schema = JSON.parse(readFileSync(schemaPath, "utf8"));
const fixture = JSON.parse(readFileSync(fixturePath, "utf8"));

const validator = new Ajv2020({ allErrors: true, strict: true });
addFormats(validator);

const validate = validator.compile(schema);
const isValid = validate(fixture);

if (!isValid) {
  console.error("Planetary-event fixture is invalid:");
  console.error(validate.errors);
  process.exit(1);
}

console.log("✓ Weather observation conforms to the Gaia planetary-event schema.");