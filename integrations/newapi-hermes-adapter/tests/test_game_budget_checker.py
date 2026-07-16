from pathlib import Path
import sys


SITE_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SITE_ROOT))
try:
    from game_plugins.base import BudgetChecker
finally:
    sys.path.pop(0)


def test_process_local_user_counter_is_reporting_only():
    checker = BudgetChecker("http://new-api", "test-token")
    checker.deduct("user-1", 10_000_000, "activity", "qq")

    allowed, reason = checker.check_user_limit("user-1", 10_000_000, "activity", "qq")

    assert allowed is True
    assert reason == ""
    assert checker._user["user-1"]["qq:activity"] == 10_000_000
