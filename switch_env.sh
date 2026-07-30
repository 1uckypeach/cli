

#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TYPES_FILE="${SCRIPT_DIR}/internal/core/types.go"
SECHEADER_FILE="${SCRIPT_DIR}/internal/cmdutil/secheader.go"

usage() {
    echo "Usage: $0 --env <boe|online|pre> --x-tt-env <value> [--build]"
    echo ""
    echo "Options:"
    echo "  --env        Set environment: 'boe' for feishu-boe.cn, 'online' for feishu.cn, 'pre' for feishu-pre.cn"
    echo "  --x-tt-env   Set x-tt-env header value. Empty string to remove the header."
    echo "  --build      Build and install lark-cli after switching environment"
    echo ""
    echo "Examples:"
    echo "  $0 --env boe --x-tt-env boe_sun_ai"
    echo "  $0 --env online --x-tt-env ''"
    echo "  $0 --env pre --x-tt-env pre_env --build"
    exit 1
}

ENV=""
XTTENV=""
BUILD=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --env)
            ENV="$2"
            shift 2
            ;;
        --x-tt-env)
            XTTENV="$2"
            shift 2
            ;;
        --build)
            BUILD=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

if [[ -z "$ENV" ]]; then
    echo "Error: --env parameter is required"
    usage
fi

if [[ "$ENV" != "boe" && "$ENV" != "online" && "$ENV" != "pre" ]]; then
    echo "Error: --env must be 'boe', 'online' or 'pre'"
    usage
fi

update_endpoints() {
    local env="$1"
    local domain
    local mcp_domain

    if [[ "$env" == "boe" ]]; then
        domain="feishu-boe.cn"
        mcp_domain="feishu-boe.cn"
    elif [[ "$env" == "pre" ]]; then
        domain="feishu-pre.cn"
        mcp_domain="feishu.cn"
    else
        domain="feishu.cn"
        mcp_domain="feishu.cn"
    fi

    echo "Updating endpoints to use domain: ${domain} (MCP: ${mcp_domain})"

    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s|feishu-boe\.cn|TEMP_DOMAIN_PLACEHOLDER|g" "$TYPES_FILE"
        sed -i '' "s|feishu-pre\.cn|TEMP_DOMAIN_PLACEHOLDER|g" "$TYPES_FILE"
        sed -i '' "s|feishu\.cn|TEMP_DOMAIN_PLACEHOLDER|g" "$TYPES_FILE"
        sed -i '' "s|TEMP_DOMAIN_PLACEHOLDER|${domain}|g" "$TYPES_FILE"
        sed -i '' "s|mcp\.${domain}|mcp.${mcp_domain}|g" "$TYPES_FILE"
    else
        sed -i "s|feishu-boe\.cn|TEMP_DOMAIN_PLACEHOLDER|g" "$TYPES_FILE"
        sed -i "s|feishu-pre\.cn|TEMP_DOMAIN_PLACEHOLDER|g" "$TYPES_FILE"
        sed -i "s|feishu\.cn|TEMP_DOMAIN_PLACEHOLDER|g" "$TYPES_FILE"
        sed -i "s|TEMP_DOMAIN_PLACEHOLDER|${domain}|g" "$TYPES_FILE"
        sed -i "s|mcp\.${domain}|mcp.${mcp_domain}|g" "$TYPES_FILE"
    fi

    echo "Endpoints updated successfully"
}

update_xtt_env_header() {
    local xtt_env="$1"

    if [[ -z "$xtt_env" ]]; then
        echo "Removing x-tt-env header..."
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' '/h\.Set("x-tt-env",/d' "$SECHEADER_FILE"
        else
            sed -i '/h\.Set("x-tt-env",/d' "$SECHEADER_FILE"
        fi
        echo "x-tt-env header removed"
    else
        echo "Setting x-tt-env header to: ${xtt_env}"
        if grep -q 'h\.Set("x-tt-env",' "$SECHEADER_FILE"; then
            if [[ "$OSTYPE" == "darwin"* ]]; then
                sed -i '' "s/h\.Set(\"x-tt-env\", \"[^\"]*\")/h.Set(\"x-tt-env\", \"${xtt_env}\")/g" "$SECHEADER_FILE"
            else
                sed -i "s/h\.Set(\"x-tt-env\", \"[^\"]*\")/h.Set(\"x-tt-env\", \"${xtt_env}\")/g" "$SECHEADER_FILE"
            fi
        else
            if [[ "$OSTYPE" == "darwin"* ]]; then
                sed -i '' "s/h\.Set(HeaderUserAgent, UserAgentValue())/h.Set(HeaderUserAgent, UserAgentValue())\n\th.Set(\"x-tt-env\", \"${xtt_env}\")/g" "$SECHEADER_FILE"
            else
                sed -i "s/h\.Set(HeaderUserAgent, UserAgentValue())/h.Set(HeaderUserAgent, UserAgentValue())\n\th.Set(\"x-tt-env\", \"${xtt_env}\")/g" "$SECHEADER_FILE"
            fi
        fi
        echo "x-tt-env header updated"
    fi

    if [[ "$xtt_env" == ppe* ]]; then
        echo "Adding x-use-ppe header (x-tt-env starts with 'ppe')..."
        if ! grep -q 'h\.Set("x-use-ppe",' "$SECHEADER_FILE"; then
            if [[ "$OSTYPE" == "darwin"* ]]; then
                sed -i '' 's/h\.Set("x-tt-env", "\(.*\)")/h.Set("x-tt-env", "\1")\'$'\n''\th.Set("x-use-ppe", "1")/g' "$SECHEADER_FILE"
            else
                sed -i 's/h\.Set("x-tt-env", "\(.*\)")/h.Set("x-tt-env", "\1")\n\th.Set("x-use-ppe", "1")/g' "$SECHEADER_FILE"
            fi
        fi
        echo "x-use-ppe header added"
    else
        if grep -q 'h\.Set("x-use-ppe",' "$SECHEADER_FILE"; then
            echo "Removing x-use-ppe header..."
            if [[ "$OSTYPE" == "darwin"* ]]; then
                sed -i '' '/h\.Set("x-use-ppe",/d' "$SECHEADER_FILE"
            else
                sed -i '/h\.Set("x-use-ppe",/d' "$SECHEADER_FILE"
            fi
            echo "x-use-ppe header removed"
        fi
    fi
}

build_and_install() {
    echo "Building and installing lark-cli..."

    rm -rf ~/.lark-cli/cache
    rm -f "${SCRIPT_DIR}/lark-cli"

    cd "$SCRIPT_DIR"
    ./build.sh

    local lark_cli_path
    lark_cli_path=$(which lark-cli 2>/dev/null || echo "")

    if [[ -n "$lark_cli_path" ]]; then
        cp "${SCRIPT_DIR}/lark-cli" "$(dirname "$lark_cli_path")/"
        echo "lark-cli installed to $(dirname "$lark_cli_path")/"
    else
        echo "Warning: lark-cli not found in PATH, binary is at ${SCRIPT_DIR}/lark-cli"
    fi

    echo "Build and install completed"
}

echo "=== Environment Switch Script ==="
echo "Environment: ${ENV}"
echo "x-tt-env: ${XTTENV:-<empty - will remove header>}"
echo "Build: ${BUILD}"
echo ""

update_endpoints "$ENV"
update_xtt_env_header "$XTTENV"

if [[ "$BUILD" == true ]]; then
    echo ""
    build_and_install
fi
echo ""
echo "=== Done ==="


