from datetime import datetime
import logging
import os
import matplotlib.pyplot as plt
import json

def setup_logger(path, name, log_level=logging.DEBUG):

    time_tag = datetime.now().strftime("%Y-%m-%d-%H-%M-%S")
    

    log_path = os.path.join(path, name + "-" + time_tag + '.log')

    logger = logging.getLogger("LOGGER")
    formatter = logging.Formatter('%(asctime)s : %(message)s')
    fileHandler = logging.FileHandler(log_path, mode='w')
    fileHandler.setFormatter(formatter)
    streamHandler = logging.StreamHandler()
    streamHandler.setFormatter(formatter)

    logger.setLevel(log_level)
    logger.addHandler(fileHandler)
    logger.addHandler(streamHandler)

    logger.info("Logger started...")


def info(text):
    logger = logging.getLogger("LOGGER")
    logger.info(text)

def debug(text):
    logger = logging.getLogger("LOGGER")
    logger.debug(text)

def error(ex, should_show_brownie_trace = False):
    logger = logging.getLogger("LOGGER")
    logger.error(str(ex))

def plot_gas_costs(costs, save_path, title=""):
    if(len(costs) == 0):
        return
    
    plt.figure()
    plt.suptitle(title)
    plt.title("mean = " + str(int(sum(costs)/ len(costs))))

    plt.xlabel("Gas cost")
    plt.ylabel("% of moves")
    plt.hist(costs, bins=20)

    file_name = f"{title}_gasCosts_histogram.png"
    file_path = os.path.join(save_path, file_name)
    plt.savefig(file_path)

def section(text, level = 0):
    indent = "    " * level
    # if level == 0 :
    #     info("***********************************")
    # else:
    #     info("")
    #     # info("")
    info(f"{indent}{text}")
    if level == 0:
        info("***********************************")
#*******************************************************************************