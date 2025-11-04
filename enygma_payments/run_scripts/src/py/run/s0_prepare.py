import os
import pathlib
import shutil

from tempfile import mkstemp
from shutil import move, copymode
import subprocess
from src.py.helpers.string_helpers import replace_in_file
from src.py.logger import info, section
from src.py.helpers.path_helpers import RecreatePath, Web3BuildPath, ProjectBuildPath, SolSourcePath
#*******************************************************************************
def s0_prepare_config_files(root_path, project_path):
	conf_path = os.path.join(root_path, "conf", "00_default_configuration.yaml")
	shutil.copy(conf_path, project_path)
	scenario_path = os.path.join(root_path, "conf", "scenarios", "00_default_scenario.yaml")
	shutil.copy(scenario_path, project_path)
#*******************************************************************************
def s0_create_enygma_project(root_path, project_path):

	info("Creating Enygma Solidity Project")
	token_path = os.path.join(project_path, "enygma")
    # filling the subproject folders
	os.makedirs(token_path)
	# info("changing directory to " + ledger_path)
	os.chdir(token_path)

	subprocess.run(["brownie", "init"])
	

	old_path = os.path.join(root_path, "..", "contracts", "enygma", "contracts", "Enygma.sol")

	print(old_path)
	new_path = os.path.join(token_path, "contracts", "Enygma.sol")
	shutil.copy(old_path, new_path)

	old_path = os.path.join(root_path, "..", "contracts", "enygma", "contracts", "CurveBabyJubJub.sol")
	new_path = os.path.join(token_path, "contracts", "CurveBabyJubJub.sol")
	shutil.copy(old_path, new_path)

	old_path = os.path.join(root_path, "..", "contracts", "enygma", "interfaces", "IEnygma.sol")
	new_path = os.path.join(token_path, "interfaces", "IEnygma.sol")
	shutil.copy(old_path, new_path)


	old_path = os.path.join(root_path, "..", "contracts", "utils", "interfaces", "IERC20.sol")
	new_path = os.path.join(token_path, "interfaces", "IERC20.sol")
	shutil.copy(old_path, new_path)

	old_path = os.path.join(root_path, "..", "contracts", "enygma", "interfaces", "IZkDvp.sol")
	new_path = os.path.join(token_path, "interfaces", "IZkDvp.sol")
	shutil.copy(old_path, new_path)

	config_path = os.path.join( token_path, "brownie-config.yaml")
	if os.path.exists(config_path):
		shutil.copy(config_path, token_path)
#*******************************************************************************
def s0_create_enygmaverifier_project(root_path, project_path):

	info("Creating Groth16 Verifier Solidity Project")
	verifier_path = os.path.join(project_path, "enygmaverifier")
    # filling the subproject folders
	os.makedirs(verifier_path)
	# info("changing directory to " + ledger_path)
	os.chdir(verifier_path)

	subprocess.run(["brownie", "init"])
	

	old_path = os.path.join(root_path, "..", "contracts", "enygmaverifier", "contracts", "EnygmaVerifier.sol")
	new_path = os.path.join(verifier_path, "contracts", "EnygmaVerifier.sol")
	shutil.copy(old_path, new_path)

	config_path = os.path.join( verifier_path, "brownie-config.yaml")
	if os.path.exists(config_path):
		shutil.copy(config_path, verifier_path)

#*******************************************************************************
# def s0_create_withdrawverifier_project(root_path, project_path):

# 	info("Creating Groth16 Withdraw Verifier Solidity Project")
# 	verifier_path = os.path.join(project_path, "withdrawverifier")
#     # filling the subproject folders
# 	os.makedirs(verifier_path)
# 	# info("changing directory to " + ledger_path)
# 	os.chdir(verifier_path)

# 	subprocess.run(["brownie", "init"])
	

# 	old_path = os.path.join(root_path, "..", "contracts", "enygmaverifier", "zkdvp", "WithdrawVerifier.sol")
# 	new_path = os.path.join(verifier_path, "contracts", "WithdrawVerifier.sol")
# 	shutil.copy(old_path, new_path)

# 	config_path = os.path.join( verifier_path, "brownie-config.yaml")
# 	if os.path.exists(config_path):
# 		shutil.copy(config_path, verifier_path)
#*******************************************************************************
def s0_create_withdrawverifier_project(root_path, project_path,k):

	info("Creating Groth16 Withdraw Verifier Solidity Project")
	verifier_path = os.path.join(project_path, f"withdrawverifier{k}")
    # filling the subproject folders
	os.makedirs(verifier_path)
	# info("changing directory to " + ledger_path)
	os.chdir(verifier_path)

	subprocess.run(["brownie", "init"])
	

	old_path = os.path.join(root_path, "..", "contracts", "enygmaverifier", "zkdvp", f"WithdrawVerifier{k}.sol")
	new_path = os.path.join(verifier_path, "contracts", f"WithdrawVerifier{k}.sol")
	shutil.copy(old_path, new_path)

	config_path = os.path.join( verifier_path, "brownie-config.yaml")
	if os.path.exists(config_path):
		shutil.copy(config_path, verifier_path)



#*******************************************************************************
def s0_create_depositverifier_project(root_path, project_path):

	info("Creating Groth16 Deposit Verifier Solidity Project")
	verifier_path = os.path.join(project_path, "depositverifier")
    # filling the subproject folders
	os.makedirs(verifier_path)
	# info("changing directory to " + ledger_path)
	os.chdir(verifier_path)

	subprocess.run(["brownie", "init"])
	

	old_path = os.path.join(root_path, "..", "contracts", "enygmaverifier", "zkdvp", "DepositVerifier.sol")
	new_path = os.path.join(verifier_path, "contracts", "DepositVerifier.sol")
	shutil.copy(old_path, new_path)

	config_path = os.path.join( verifier_path, "brownie-config.yaml")
	if os.path.exists(config_path):
		shutil.copy(config_path, verifier_path)



#*******************************************************************************
def s0_prepare(root_path, project_name, banks_conf):
	section("[[PREPARE]]")


	project_path = ProjectBuildPath(root_path, project_name)
	RecreatePath(project_path)

	web3_build_path = Web3BuildPath(root_path, project_name)
	RecreatePath(web3_build_path)

	s0_create_enygma_project(root_path, web3_build_path)
	s0_create_enygmaverifier_project(root_path, web3_build_path)
	# s0_create_withdrawverifier_project(root_path, web3_build_path)
	for i in range(7):
		if i ==0:
			continue
		else:
			s0_create_withdrawverifier_project(root_path, web3_build_path,i)
		
	s0_create_depositverifier_project(root_path, web3_build_path)
	# s0_prepare_config_files(root_path, project_path)

