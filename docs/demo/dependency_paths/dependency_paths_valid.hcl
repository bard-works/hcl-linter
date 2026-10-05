# Demo: dependency_paths rule - VALID
# config_path points to existing directories (units/vpc and units/database
# next to this file).

dependency "vpc" {
  config_path = "./units/vpc"
}

dependency "db" {
  config_path = "./units/database"
}
