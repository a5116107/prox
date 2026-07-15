import json
import os
from game_config_manager import get_config_manager


def main():
    cm = get_config_manager()
    result = cm._push_local_snapshot(
        reason=os.environ.get("BOOTSTRAP_REASON", "bootstrap_local_config")
    )
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
