from unittest.mock import patch, MagicMock
from click.testing import CliRunner
import os
from main import main
from lazyworktree.config import AppConfig

def test_cli_debug_log_option():
    runner = CliRunner()
    
    # Mock GitWtStatus to capture the config passed to it
    with patch("main.GitWtStatus") as MockApp:
        mock_instance = MagicMock()
        mock_instance.run.return_value = None
        MockApp.return_value = mock_instance
        
        # Run main with --debug-log
        result = runner.invoke(main, ["--debug-log", "/tmp/cli_debug.log"])
        
        assert result.exit_code == 0
        
        # Verify GitWtStatus was initialized
        assert MockApp.called
        
        # Check the config passed to GitWtStatus
        _, kwargs = MockApp.call_args
        config = kwargs.get("config")
        assert isinstance(config, AppConfig)
        assert config.debug_log == os.path.expanduser("/tmp/cli_debug.log")

def test_cli_debug_log_overrides_config(tmp_path):
    runner = CliRunner()
    
    # Create a dummy config file
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    config_file = config_dir / "lazyworktree" / "config.yaml"
    config_file.parent.mkdir()
    config_file.write_text("debug_log: /tmp/config_debug.log\n", encoding="utf-8")
    
    # Mock XDG_CONFIG_HOME to point to our temp config
    env = {"XDG_CONFIG_HOME": str(config_dir)}
    
    with patch("main.GitWtStatus") as MockApp, patch.dict(os.environ, env):
        mock_instance = MagicMock()
        mock_instance.run.return_value = None
        MockApp.return_value = mock_instance
        
        # Run main with --debug-log
        result = runner.invoke(main, ["--debug-log", "/tmp/cli_override.log"])
        
        assert result.exit_code == 0
        
        # Verify the CLI arg overrode the config file
        _, kwargs = MockApp.call_args
        config = kwargs.get("config")
        assert config.debug_log == os.path.expanduser("/tmp/cli_override.log")

