import yaml
import gym
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
from stable_baselines3 import PPO, DQN, A2C
from stable_baselines3.common.vec_env import DummyVecEnv

# === Load YAML Config ===
with open("config.yaml", "r") as f:
    config = yaml.safe_load(f)

# === Agent Class ===
class RLAgent:
    def __init__(self, algo_name, env_name, total_timesteps, params):
        self.algo_name = algo_name
        self.env_name = env_name
        self.total_timesteps = total_timesteps
        self.params = params
        self.env = DummyVecEnv([lambda: gym.make(env_name)])
        self.model = self._initialize_model()

    def _initialize_model(self):
        algo_map = {
            "PPO": PPO,
            "DQN": DQN,
            "A2C": A2C
        }
        model_class = algo_map[self.algo_name]
        return model_class("MlpPolicy", self.env, verbose=0, **self.params)

    def train(self):
        print(f"\n🚀 Training {self.algo_name}...")
        self.model.learn(total_timesteps=self.total_timesteps)

    def evaluate(self, n_episodes):
        rewards = []
        for _ in range(n_episodes):
            obs = self.env.reset()
            done = [False]
            total_reward = 0
            while not done[0]:
                action, _ = self.model.predict(obs, deterministic=True)
                obs, reward, done, info = self.env.step(action)
                total_reward += reward[0]
            rewards.append(total_reward)
        mean_reward = np.mean(rewards)
        std_reward = np.std(rewards)
        return mean_reward, std_reward

# === Train & Evaluate ===
results = []

for algo in config["algorithms"]:
    agent = RLAgent(
        algo_name=algo["name"],
        env_name=config["env_name"],
        total_timesteps=config["total_timesteps"],
        params=algo.get("params", {})
    )
    agent.train()
    mean_reward, std_reward = agent.evaluate(n_episodes=config["evaluation_episodes"])
    results.append({
        "Algorithm": algo["name"],
        "Mean Reward": mean_reward,
        "Std Reward": std_reward
    })

# === Show Results ===
df_results = pd.DataFrame(results)
print("\n📊 RL Algorithm Performance on CartPole:")
print(df_results)

# === Plot ===
plt.figure(figsize=(8, 5))
plt.bar(df_results["Algorithm"], df_results["Mean Reward"], yerr=df_results["Std Reward"], capsize=5)
plt.ylabel("Mean Reward")
plt.title("Performance of PPO, DQN, A2C on CartPole")
plt.grid(True)
plt.tight_layout()
plt.show()
