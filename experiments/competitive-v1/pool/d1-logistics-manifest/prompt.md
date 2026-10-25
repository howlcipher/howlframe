# Freight Container Packing Optimizer

Build a CLI tool that parses shipping package manifests, validates package records against logistics rules, and packs valid packages into single-destination freight containers obeying weight and hazardous material limits.

## CLI Interface

The executable is located at `./run`. It must implement the `pack` subcommand:

```bash
./run pack --packages <packages_csv> --rules <rules_json> --output <output_json>
```

Arguments:
- `pack`: Required subcommand. If absent or different, exit with code `1`.
- `--packages <packages_csv>`: Path to a CSV file containing package records. Required.
- `--rules <rules_json>`: Path to a JSON configuration file defining packing rules. Required.
- `--output <output_json>`: Path where the resulting container packing manifest will be saved. Required.

Options may be passed in any order.

### Exit Codes
- `0`: Success, output manifest written.
- `1`: Invalid CLI arguments, missing required options, or unrecognized subcommand.
- `2`: Either the `--packages` file or `--rules` file does not exist or cannot be read.

## Input File Formats

### 1. Rules Configuration (`--rules`)
A JSON file with constraints:
- `"max_container_weight_kg"`: positive float (maximum weight of packages allowed in a single container).
- `"max_hazard_items_per_container"`: non-negative integer (maximum number of hazardous packages in one container).
- `"allowed_destinations"`: array of uppercase string destination names.

Example `rules.json`:
```json
{
  "max_container_weight_kg": 1000.0,
  "max_hazard_items_per_container": 2,
  "allowed_destinations": ["NORTH", "SOUTH", "EAST", "WEST"]
}
```

### 2. Package Manifest (`--packages`)
A standard comma-separated CSV file with header:
`package_id,destination,weight_kg,is_hazardous,priority`

- `package_id`: non-empty string.
- `destination`: string matching an allowed destination.
- `weight_kg`: positive numeric float.
- `is_hazardous`: case-insensitive boolean string (`"true"` or `"false"`).
- `priority`: integer between `1` and `5` inclusive (where `1` is highest priority and `5` is lowest).

Example `packages.csv`:
```csv
package_id,destination,weight_kg,is_hazardous,priority
PKG-101,NORTH,450.0,false,1
PKG-102,NORTH,600.0,true,2
PKG-103,SOUTH,120.0,true,1
```

## Validation & Rejection Rules

Check each row from the CSV file. A package is rejected if:
1. `destination` is not in `allowed_destinations` -> reason: `"invalid_destination"`.
2. `weight_kg <= 0` or `weight_kg > max_container_weight_kg` -> reason: `"overweight"`.
3. Non-numeric weight, invalid `priority` (not an integer 1..5), non-boolean `is_hazardous`, or missing/empty `package_id` -> reason: `"invalid_fields"`.

If multiple rejection conditions apply, evaluate in order: `"invalid_fields"` -> `"invalid_destination"` -> `"overweight"`.
All rejected items must be collected in `"rejected_packages"`, sorted by `package_id` ascending.

## Packing Algorithm

Only valid packages are packed.
1. Sort valid packages by:
   - Primary: `priority` ascending (`1` before `2`).
   - Secondary: `weight_kg` descending (heavier items packed first).
   - Tertiary: `package_id` ascending (lexicographical code-point order).
2. Containers are maintained in an ordered list, identified sequentially as `"CONT-1"`, `"CONT-2"`, `"CONT-3"`, etc.
   - Each container is dedicated to exactly one destination (determined by the first package placed in it).
3. For each package in the sorted list:
   - Check existing containers from first to last:
     - Container destination must match the package destination.
     - `container.total_weight_kg + package.weight_kg <= max_container_weight_kg`
     - If `package.is_hazardous == true`: `container.hazard_count + 1 <= max_hazard_items_per_container`
   - If a compatible container is found: assign the package to that container.
   - If no existing container can take the package: allocate a new container (e.g. `"CONT-1"`, `"CONT-2"`), assign its destination to the package's destination, and place the package inside.

## Output Format (`--output`)

Write the final JSON to `<output_json>`:

```json
{
  "total_containers": 1,
  "total_packed_packages": 2,
  "containers": [
    {
      "container_id": "CONT-1",
      "destination": "NORTH",
      "total_weight_kg": 750.0,
      "hazard_count": 1,
      "package_ids": ["PKG-101", "PKG-102"]
    }
  ],
  "rejected_packages": [
    {
      "package_id": "PKG-BAD",
      "reason": "invalid_destination"
    }
  ]
}
```

Format numbers rounded to 2 decimal places in `total_weight_kg`.
Containers must appear in container_id order (`CONT-1`, `CONT-2`, ...).
`rejected_packages` must be sorted by `package_id` ascending.
