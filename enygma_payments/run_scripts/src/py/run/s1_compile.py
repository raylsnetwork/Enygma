import os
import subprocess
import shutil

from src.py.logger import info, section
from src.py.helpers.path_helpers import Web3BuildPath, DeleteFilesByExtentions
from src.py.helpers.json_helpers import extract_abi_from_json
from src.py.helpers.string_helpers import replace_in_file
from distutils.dir_util import copy_tree
#*******************************************************************************
def s1_compile_enygma(project_path):
    section("[Compiling Enygma]", 1)

    enygma_path = os.path.join(project_path, "enygma")
    os.chdir(enygma_path)
    subprocess.run(["brownie", "compile", "all"])
#*******************************************************************************
def s1_compile_enygmaverifier(project_path):
    section("[Compiling EnygmaVerifier]", 1)

    enygmaverifier_path = os.path.join(project_path, "enygmaverifier")
    os.chdir(enygmaverifier_path)
    subprocess.run(["brownie", "compile", "all"])
#*******************************************************************************
# def s1_compile_withdrawverifier(project_path):
#     section("[Compiling EnygmaVerifier]", 1)

#     withdrawverifier_path = os.path.join(project_path, "withdrawverifier")
#     os.chdir(withdrawverifier_path)
#     subprocess.run(["brownie", "compile", "all"])
# #*******************************************************************************
def s1_compile_withdrawverifier(project_path,k):
    section("[Compiling EnygmaVerifier]", 1)

    withdrawverifier_path = os.path.join(project_path, f"withdrawverifier{k}")
    os.chdir(withdrawverifier_path)
    subprocess.run(["brownie", "compile", "all"])

#*******************************************************************************
def s1_compile_depositverifier(project_path):
    section("[Compiling EnygmaVerifier]", 1)

    depositverifier_path = os.path.join(project_path, "depositverifier")
    os.chdir(depositverifier_path)
    subprocess.run(["brownie", "compile", "all"])
#*******************************************************************************
def s1_compile(root_path, project_name, banks_conf):
    section("[[COMPILE]]")

    web3_path = Web3BuildPath(root_path, project_name)

    s1_compile_enygma(web3_path)
    s1_compile_enygmaverifier(web3_path)
    # s1_compile_withdrawverifier(web3_path)
    for i in range(7):
            if i ==0:
                continue
            else:
                s1_compile_withdrawverifier(web3_path,i)
		
    s1_compile_depositverifier(web3_path)
