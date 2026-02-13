using System.Collections.Generic;
using MyCorp.Core;

namespace MyCorp.App {
    public class UserManager : BaseManager {
        public ILogger _logger;
        public string AppName { get; set; }

        public UserManager(ILogger logger) {
            _logger = logger;
            _logger.Log("Initializing...");
        }

        public void ProcessUsers() {
             var users = new List<string>();
             users.Add("Alice");
             
             var helper = new UserHelper();
             helper.Validate(users);
        }
    }
}
