import os
import shutil
import pathlib

def DeleteFilesByExtentions(input_path, extensions=[]):

	files_list = os.listdir(input_path)

	for item in files_list:
		for ex in extensions:
			if item.endswith("." + ex):
				os.remove(os.path.join(input_path, item))

def findFilesByExtention(input_path, extension="pgn"):
	file_list = []
	if os.path.isdir(input_path):
		for root, dirs, files in os.walk(input_path):
			for file in files:
				if file.endswith("." + extension):
					file_list.append(os.path.join(root, file))
	else:
		file_list.append(input_path)

	return file_list

def RootPath():
	return str(pathlib.Path(__file__).parent.parent.parent.parent.resolve())

def BuildPath(root_path):
	return os.path.join(root_path, "build")

def ProjectBuildPath(root_path, project_name):
	return os.path.join(root_path, "build", project_name)

def Web3BuildPath(root_path, project_name):
	return os.path.join(root_path, "build", project_name, "web3")

def ConfigPath(root_path):
	return os.path.join(root_path, "conf")

def Web3SourcePath(root_path):
	return os.path.join(root_path, "..", "contracts")

def SolSourcePath(root_path):
	return os.path.join(root_path, "..", "contracts")

def TokenJsonPath(root_path, project_name):
	return os.path.join(Web3BuildPath(root_path, project_name), "enygma", "build", "contracts", "Enygma.json")

def VerifierJsonPath(root_path, project_name):
	return os.path.join(Web3BuildPath(root_path, project_name), "enygmaverifier", "build", "contracts", "Verifier.json")


def WithdrawVerifierJsonPath(root_path, project_name,k):
	return os.path.join(Web3BuildPath(root_path, project_name), f"withdrawverifier{k}", "build", "contracts", "Verifier.json")

def DepositVerifierJsonPath(root_path, project_name):
	return os.path.join(Web3BuildPath(root_path, project_name), "depositverifier", "build", "contracts", "Verifier.json")

def PlotSavePath(root_path):
	plot_path = os.path.join(SavePath(root_path), "plots")
	return plot_path

def DefaultConfiguration(root_path):
	return os.path.join(root_path, "conf", "00_default_configuration.yaml")

def RecreatePath(path):
	if os.path.exists(path):
		shutil.rmtree(path)
	os.makedirs(path)
