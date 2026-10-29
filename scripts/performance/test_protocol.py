import unittest
from longevity import visible_text

class ProtocolTests(unittest.TestCase):
 def test_metadata_is_not_an_answer_or_first_token(self):
  self.assertEqual(visible_text({'finish_reason':'memory_used','content':'9'}),'')
  self.assertEqual(visible_text({'finish_reason':'heartbeat','content':''}),'')
  self.assertEqual(visible_text({'finish_reason':'agent_event','content':'{"type":"tool"}'}),'')
  self.assertEqual(visible_text({'content':'First actual word'}),'First actual word')
 def test_terminal_text_is_preserved(self):
  self.assertEqual(visible_text({'finish_reason':'stop','content':'last word','done':True}),'last word')

if __name__=='__main__':unittest.main()
