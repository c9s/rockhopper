#!/bin/bash
set -e

usage() {
    echo "Usage: $0 [-t type] <migration_name>"
    echo ""
    echo "Create migration files for all rockhopper_*.yaml configs found in the project root."
    echo ""
    echo "Options:"
    echo "  -t type    Migration type (default: sql)"
    echo ""
    echo "Example:"
    echo "  $0 add_pnl_column"
    echo "  $0 -t sql add_trades_table"
    exit 1
}

TYPE="sql"

while getopts "t:h" opt; do
    case $opt in
        t) TYPE="$OPTARG" ;;
        h) usage ;;
        *) usage ;;
    esac
done
shift $((OPTIND - 1))

if [ -z "$1" ]; then
    echo "Error: migration name is required"
    usage
fi

MIGRATION_NAME="$1"

# Find project root by looking for go.mod
PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

CONFIGS=$(find "$PROJECT_ROOT" -maxdepth 1 -name 'rockhopper_*.yaml' | sort)

if [ -z "$CONFIGS" ]; then
    echo "Error: no rockhopper_*.yaml config files found in $PROJECT_ROOT"
    exit 1
fi

echo "Creating migration '$MIGRATION_NAME' (type: $TYPE)"
echo ""

for config in $CONFIGS; do
    config_name=$(basename "$config")
    echo ">> $config_name"
    rockhopper --config "$config" create --type "$TYPE" "$MIGRATION_NAME"
    echo ""
done

echo "Done. Remember to edit all migration files — SQL syntax may differ between dialects."
